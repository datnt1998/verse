package storageimport

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Dump evidence contains aggregates only, never row data. COPY column lists
// select the timestamptz maxima that must survive a restore unchanged.
type backupTable struct {
	name    string
	columns map[string]uint32
}

type backupTableStats struct {
	name        string
	rows        int64
	timestamps  map[string]pgtype.Timestamptz
	copyColumns []string
}

type backupStats struct {
	schema string
	tables []backupTableStats
}

func backupTables(ctx context.Context, q querier, schema string) ([]backupTable, error) {
	rows, err := q.Query(ctx, `
SELECT c.relname, a.attname, a.atttypid
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
WHERE n.nspname = $1 AND c.relkind = 'r'
ORDER BY c.relname, a.attnum`, schema)
	if err != nil {
		return nil, fmt.Errorf("đọc danh sách bảng backup: %w", err)
	}
	defer rows.Close()
	var tables []backupTable
	for rows.Next() {
		var name string
		var column *string
		var oid *uint32
		if err := rows.Scan(&name, &column, &oid); err != nil {
			return nil, fmt.Errorf("đọc cấu trúc bảng backup: %w", err)
		}
		if len(tables) == 0 || tables[len(tables)-1].name != name {
			tables = append(tables, backupTable{name: name, columns: map[string]uint32{}})
		}
		if column != nil && oid != nil {
			tables[len(tables)-1].columns[*column] = *oid
		}
	}
	return tables, rows.Err()
}

// sqlTokens recognizes identifier quoting and punctuation in pg_dump's COPY
// header and TOC identifier tail. It never parses or evaluates SQL values.
func sqlTokens(s string) ([]string, error) {
	var tokens []string
	for i := 0; i < len(s); {
		if unicode.IsSpace(rune(s[i])) {
			i++
			continue
		}
		if strings.ContainsRune(".(),;", rune(s[i])) {
			tokens = append(tokens, s[i:i+1])
			i++
			continue
		}
		if s[i] == '"' {
			i++
			var name strings.Builder
			closed := false
			for i < len(s) {
				if s[i] == '"' {
					if i+1 < len(s) && s[i+1] == '"' {
						name.WriteByte('"')
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				name.WriteByte(s[i])
				i++
			}
			if !closed {
				return nil, errors.New("identifier có dấu ngoặc kép chưa đóng")
			}
			tokens = append(tokens, name.String())
			continue
		}
		start := i
		for i < len(s) && !unicode.IsSpace(rune(s[i])) && !strings.ContainsRune(".(),;\"", rune(s[i])) {
			i++
		}
		if i == start {
			return nil, errors.New("identifier COPY không hợp lệ")
		}
		tokens = append(tokens, s[start:i])
	}
	return tokens, nil
}

func copyHeader(line string) (schema, table string, columns []string, err error) {
	tokens, err := sqlTokens(line)
	if err != nil || len(tokens) < 9 || tokens[0] != "COPY" || tokens[2] != "." || tokens[4] != "(" {
		return "", "", nil, errors.New("header COPY không hợp lệ")
	}
	schema, table = tokens[1], tokens[3]
	i := 5
	for i < len(tokens) && tokens[i] != ")" {
		columns = append(columns, tokens[i])
		i++
		if i < len(tokens) && tokens[i] == "," {
			i++
		} else if i >= len(tokens) || tokens[i] != ")" {
			return "", "", nil, errors.New("danh sách cột COPY không hợp lệ")
		}
	}
	if i+4 != len(tokens) || tokens[i] != ")" || tokens[i+1] != "FROM" || tokens[i+2] != "stdin" || tokens[i+3] != ";" {
		return "", "", nil, errors.New("backup chỉ nhận COPY FROM stdin")
	}
	return schema, table, columns, nil
}

func laterTimestamp(a, b pgtype.Timestamptz) bool {
	if !a.Valid {
		return false
	}
	if !b.Valid {
		return true
	}
	if a.InfinityModifier != b.InfinityModifier {
		return a.InfinityModifier > b.InfinityModifier
	}
	return a.InfinityModifier == pgtype.Finite && a.Time.After(b.Time)
}

func sameTimestamp(a, b pgtype.Timestamptz) bool {
	if a.Valid != b.Valid {
		return false
	}
	return !a.Valid || (a.InfinityModifier == b.InfinityModifier &&
		(a.InfinityModifier != pgtype.Finite || a.Time.Equal(b.Time)))
}

// readDumpStats consumes the entire data-only output, including ignored
// schemas. It stores one line at a time and only the required aggregates.
func readDumpStats(r io.Reader, schema string, tables []backupTable) (*backupStats, error) {
	stats := &backupStats{schema: schema, tables: make([]backupTableStats, len(tables))}
	byName := make(map[string]int, len(tables))
	for i, t := range tables {
		byName[t.name] = i
		stats.tables[i] = backupTableStats{name: t.name, timestamps: map[string]pgtype.Timestamptz{}}
	}
	seen := make(map[string]bool, len(tables))
	reader := bufio.NewReader(r)
	decoder := pgtype.NewMap()
	inCopy := false
	active := -1
	var timestampIndexes []int
	var columns []string
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, errors.New("đọc dữ liệu COPY từ archive thất bại")
		}
		if len(line) == 0 && errors.Is(readErr, io.EOF) {
			break
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if inCopy {
			if line == `\.` {
				inCopy = false
				active = -1
			} else if active >= 0 {
				t := &stats.tables[active]
				fields := strings.Split(line, "\t")
				if len(fields) != len(columns) {
					return nil, fmt.Errorf("bảng %s.%s: số cột COPY không hợp lệ", schema, t.name)
				}
				t.rows++
				for _, index := range timestampIndexes {
					var value pgtype.Timestamptz
					var text []byte
					if fields[index] != `\N` {
						text = []byte(fields[index])
					}
					if err := decoder.Scan(pgtype.TimestamptzOID, pgtype.TextFormatCode, text, &value); err != nil {
						return nil, fmt.Errorf("bảng %s.%s: timestamp cột %s không hợp lệ", schema, t.name, columns[index])
					}
					if laterTimestamp(value, t.timestamps[columns[index]]) {
						t.timestamps[columns[index]] = value
					}
				}
			}
		} else if strings.HasPrefix(line, "COPY ") {
			copySchema, name, copyColumns, err := copyHeader(line)
			if err != nil {
				return nil, errors.New("archive có header COPY không hợp lệ")
			}
			inCopy = true
			active = -1
			columns = copyColumns
			timestampIndexes = timestampIndexes[:0]
			if index, ok := byName[name]; ok && copySchema == schema {
				if seen[name] {
					return nil, fmt.Errorf("bảng %s.%s: COPY bị lặp", schema, name)
				}
				seen[name] = true
				active = index
				stats.tables[index].copyColumns = copyColumns
				known := map[string]bool{}
				for n, column := range copyColumns {
					oid, exists := tables[index].columns[column]
					if !exists || known[column] {
						return nil, fmt.Errorf("bảng %s.%s: cột COPY không khớp cấu trúc đích", schema, name)
					}
					known[column] = true
					if oid == pgtype.TimestamptzOID {
						timestampIndexes = append(timestampIndexes, n)
						stats.tables[index].timestamps[column] = pgtype.Timestamptz{}
					}
				}
			}
		} else if strings.HasPrefix(strings.TrimSpace(line), "INSERT INTO ") {
			return nil, errors.New("archive dùng INSERT; chỉ nhận dữ liệu COPY (không --inserts/--column-inserts)")
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
	}
	if inCopy {
		return nil, errors.New("archive thiếu dấu kết thúc COPY")
	}
	for _, table := range tables {
		if !seen[table.name] {
			return nil, fmt.Errorf("bảng %s.%s: thiếu COPY FROM stdin; không nhận dump --inserts/--column-inserts", schema, table.name)
		}
	}
	return stats, nil
}

func cutoverLockError(name string, err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return fmt.Errorf("khoá bảng %s thất bại — rollback", name)
	}
	if pgErr.Code == "55P03" {
		return fmt.Errorf("không thể khoá bảng %s vì còn giao dịch đang hoạt động (có thể là autovacuum/ANALYZE) — dừng mọi app/worker kết nối database rồi thử lại; kiểm tra pg_locks/pg_stat_activity nếu vẫn bị chặn; rollback", name)
	}
	// The SQLSTATE tells a missing privilege from a missing table and
	// carries no data.
	return fmt.Errorf("khoá bảng %s thất bại (SQLSTATE %s) — rollback", name, pgErr.Code)
}

// lockLiveTables excludes writers until commit, including writers that never
// acquire auth_invariant_lock. ONLY keeps inheritance locking table-by-table.
func (s *backupStats) lockLiveTables(ctx context.Context, tx pgx.Tx) error {
	var schema string
	if err := tx.QueryRow(ctx, `SELECT n.nspname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.oid='users'::regclass`).Scan(&schema); err != nil {
		return errors.New("phân giải schema khi khoá bảng backup thất bại")
	}
	if schema != s.schema {
		return errors.New("schema chứa users đã đổi sau kiểm chứng backup")
	}
	tables, err := backupTables(ctx, tx, schema)
	if err != nil {
		return err
	}
	for _, table := range tables {
		name := pgx.Identifier{schema, table.name}.Sanitize()
		if _, err := tx.Exec(ctx, "LOCK TABLE ONLY "+name+" IN EXCLUSIVE MODE NOWAIT"); err != nil {
			return cutoverLockError(name, err)
		}
	}
	return nil
}

func (s *backupStats) compareLive(ctx context.Context, q querier) error {
	var schema string
	if err := q.QueryRow(ctx, `SELECT n.nspname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.oid='users'::regclass`).Scan(&schema); err != nil {
		return fmt.Errorf("phân giải schema khi đối chiếu backup: %w", err)
	}
	if schema != s.schema {
		return errors.New("schema chứa users đã đổi sau kiểm chứng backup")
	}
	tables, err := backupTables(ctx, q, schema)
	if err != nil {
		return err
	}
	if len(tables) != len(s.tables) {
		return fmt.Errorf("backup không khớp schema %s: danh sách bảng đã đổi", schema)
	}
	for i, expected := range s.tables {
		if tables[i].name != expected.name {
			return fmt.Errorf("backup không khớp bảng %s.%s: danh sách bảng đã đổi", schema, expected.name)
		}
		query := "SELECT count(*)"
		var columns []string
		for _, column := range expected.copyColumns {
			if _, timestamp := expected.timestamps[column]; timestamp {
				if tables[i].columns[column] != pgtype.TimestamptzOID {
					return fmt.Errorf("backup không khớp bảng %s.%s: timestamp cột %s đã đổi kiểu", schema, expected.name, column)
				}
				columns = append(columns, column)
				query += ", max(" + pgx.Identifier{column}.Sanitize() + ")"
			}
		}
		query += " FROM ONLY " + pgx.Identifier{schema, expected.name}.Sanitize()
		var count int64
		values := make([]pgtype.Timestamptz, len(columns))
		dest := make([]any, len(columns)+1)
		dest[0] = &count
		for j := range values {
			dest[j+1] = &values[j]
		}
		if err := q.QueryRow(ctx, query).Scan(dest...); err != nil {
			return fmt.Errorf("đối chiếu backup bảng %s.%s thất bại", schema, expected.name)
		}
		if count != expected.rows {
			return fmt.Errorf("backup không khớp bảng %s.%s: số dòng khác", schema, expected.name)
		}
		for j, column := range columns {
			if !sameTimestamp(values[j], expected.timestamps[column]) {
				return fmt.Errorf("backup không khớp bảng %s.%s: timestamp cột %s khác", schema, expected.name, column)
			}
		}
	}
	return nil
}
