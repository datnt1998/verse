package bootstrap

import (
	"strings"
	"testing"
	"time"
)

// TestStorageConfig covers the env contract phase 02 locks:
// file is the only silent default, postgres requires an explicit DSN, an
// invalid driver fails instead of falling back to file, and no error ever
// echoes the DSN.
func TestStorageConfig(t *testing.T) {
	clear := func(t *testing.T) {
		t.Helper()
		for _, key := range []string{
			"STORE_DRIVER", "DATABASE_URL", "MIGRATION_DATABASE_URL",
			"DB_MAX_CONNS", "DB_CONNECT_TIMEOUT",
		} {
			t.Setenv(key, "")
		}
	}

	t.Run("default_is_file", func(t *testing.T) {
		clear(t)
		cfg, err := StorageConfigFromEnv()
		if err != nil {
			t.Fatalf("default env: %v", err)
		}
		if cfg.Driver != StoreDriverFile {
			t.Fatalf("default driver = %q, want file", cfg.Driver)
		}
	})

	t.Run("explicit_file", func(t *testing.T) {
		clear(t)
		t.Setenv("STORE_DRIVER", "file")
		cfg, err := StorageConfigFromEnv()
		if err != nil {
			t.Fatalf("explicit file: %v", err)
		}
		if cfg.Driver != StoreDriverFile {
			t.Fatalf("driver = %q, want file", cfg.Driver)
		}
	})

	t.Run("postgres_requires_database_url", func(t *testing.T) {
		clear(t)
		t.Setenv("STORE_DRIVER", "postgres")
		_, err := StorageConfigFromEnv()
		if err == nil {
			t.Fatal("postgres without DATABASE_URL must fail, not fall back to file")
		}
		if !strings.Contains(err.Error(), "DATABASE_URL") {
			t.Fatalf("error should name the missing variable, got %v", err)
		}
	})

	t.Run("postgres_explicit", func(t *testing.T) {
		clear(t)
		t.Setenv("STORE_DRIVER", "postgres")
		t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/app")
		cfg, err := StorageConfigFromEnv()
		if err != nil {
			t.Fatalf("postgres env: %v", err)
		}
		if cfg.Driver != StoreDriverPostgres {
			t.Fatalf("driver = %q, want postgres", cfg.Driver)
		}
	})

	t.Run("invalid_driver_fails", func(t *testing.T) {
		clear(t)
		t.Setenv("STORE_DRIVER", "sqlite")
		if _, err := StorageConfigFromEnv(); err == nil {
			t.Fatal("unknown driver must fail, not fall back to file")
		}
	})

	t.Run("migration_dsn_is_separate", func(t *testing.T) {
		clear(t)
		t.Setenv("STORE_DRIVER", "postgres")
		t.Setenv("DATABASE_URL", "postgres://app:p@db:5432/app")
		t.Setenv("MIGRATION_DATABASE_URL", "postgres://mig:p@db:5432/app")
		cfg, err := StorageConfigFromEnv()
		if err != nil {
			t.Fatalf("env: %v", err)
		}
		if cfg.MigrationDSN() != "postgres://mig:p@db:5432/app" {
			t.Fatalf("migration DSN = %q, want dedicated role", cfg.MigrationDSN())
		}
	})

	t.Run("migration_dsn_falls_back_to_app_dsn", func(t *testing.T) {
		clear(t)
		t.Setenv("STORE_DRIVER", "postgres")
		t.Setenv("DATABASE_URL", "postgres://app:p@db:5432/app")
		cfg, err := StorageConfigFromEnv()
		if err != nil {
			t.Fatalf("env: %v", err)
		}
		if cfg.MigrationDSN() != "postgres://app:p@db:5432/app" {
			t.Fatalf("migration DSN = %q, want fallback to DATABASE_URL", cfg.MigrationDSN())
		}
	})

	t.Run("database_url_without_driver_does_not_leak_into_file_mode", func(t *testing.T) {
		clear(t)
		t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/app")
		cfg, err := StorageConfigFromEnv()
		if err != nil {
			t.Fatalf("file mode: %v", err)
		}
		if cfg.Driver != StoreDriverFile {
			t.Fatalf("driver = %q; DATABASE_URL must not silently activate postgres", cfg.Driver)
		}
	})

	t.Run("pool_bounds_parse", func(t *testing.T) {
		clear(t)
		t.Setenv("STORE_DRIVER", "postgres")
		t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/app")
		t.Setenv("DB_MAX_CONNS", "12")
		t.Setenv("DB_CONNECT_TIMEOUT", "3s")
		cfg, err := StorageConfigFromEnv()
		if err != nil {
			t.Fatalf("env: %v", err)
		}
		if cfg.MaxConns != 12 {
			t.Fatalf("MaxConns = %d, want 12", cfg.MaxConns)
		}
		if cfg.ConnectTimeout != 3*time.Second {
			t.Fatalf("ConnectTimeout = %v, want 3s", cfg.ConnectTimeout)
		}
	})

	t.Run("invalid_pool_bounds_fail", func(t *testing.T) {
		clear(t)
		t.Setenv("STORE_DRIVER", "postgres")
		t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/app")
		t.Setenv("DB_MAX_CONNS", "abc")
		if _, err := StorageConfigFromEnv(); err == nil {
			t.Fatal("invalid DB_MAX_CONNS must fail")
		}
	})

	t.Run("pool_bounds_consume_the_whole_value", func(t *testing.T) {
		// fmt.Sscanf used to accept "8 conns" as 8; a partially valid value
		// is a typo and must fail startup.
		for _, v := range []string{"8 conns", "8x", "1.5", "-4", "0", "101"} {
			clear(t)
			t.Setenv("STORE_DRIVER", "postgres")
			t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/app")
			t.Setenv("DB_MAX_CONNS", v)
			if _, err := StorageConfigFromEnv(); err == nil {
				t.Fatalf("DB_MAX_CONNS=%q must fail (whole-string, bounded 1..100)", v)
			}
		}
	})

	t.Run("connect_timeout_is_bounded", func(t *testing.T) {
		for _, v := range []string{"0s", "-1s", "500ms", "10m"} {
			clear(t)
			t.Setenv("STORE_DRIVER", "postgres")
			t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/app")
			t.Setenv("DB_CONNECT_TIMEOUT", v)
			if _, err := StorageConfigFromEnv(); err == nil {
				t.Fatalf("DB_CONNECT_TIMEOUT=%q must fail (1s..2m)", v)
			}
		}
	})

	t.Run("errors_never_print_dsn", func(t *testing.T) {
		clear(t)
		t.Setenv("STORE_DRIVER", "postgres")
		t.Setenv("DATABASE_URL", "postgres://user:Sup3rSecret@db:5432/app")
		t.Setenv("DB_MAX_CONNS", "nope")
		_, err := StorageConfigFromEnv()
		if err == nil {
			t.Fatal("invalid env must fail")
		}
		if strings.Contains(err.Error(), "Sup3rSecret") || strings.Contains(err.Error(), "user:Sup3rSecret@") {
			t.Fatalf("error leaked DSN secret: %v", err)
		}
	})
}

// TestStorageConfigDSNNeverExposed checks the String/Log form of the config
// redacts credentials for diagnostics.
func TestStorageConfigDSNNeverExposed(t *testing.T) {
	t.Setenv("STORE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgres://user:Sup3rSecret@db:5432/app")
	cfg, err := StorageConfigFromEnv()
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	safe := cfg.SafeSummary()
	if strings.Contains(safe, "Sup3rSecret") || strings.Contains(safe, "user:Sup3rSecret@") {
		t.Fatalf("SafeSummary leaked secret: %s", safe)
	}
	if !strings.Contains(safe, "postgres") {
		t.Fatalf("SafeSummary should name the driver: %s", safe)
	}
}
