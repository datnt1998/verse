import { useCallback, useEffect, useRef, useState } from 'react'
import { api, ApiError, type Run, type ScriptSummary, type User } from './api'
import { briefSummary, formatCost, formatDuration, formatTime, hasActiveRun, statusLabel, validateBrief } from './lib'

type View = { name: 'runs' } | { name: 'script'; id: string }

export default function App() {
  const [user, setUser] = useState<User | null>(null)
  const [setupNeeded, setSetupNeeded] = useState<boolean | null>(null)
  const [view, setView] = useState<View>({ name: 'runs' })
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .setupStatus()
      .then((s) => setSetupNeeded(s.needed))
      .catch(() => setSetupNeeded(false))
    api
      .me()
      .then((m) => setUser(m.user))
      .catch(() => setUser(null))
  }, [])

  const onAuthed = useCallback((u: User) => {
    setUser(u)
    setView({ name: 'runs' })
  }, [])

  if (setupNeeded === null) return <div className="page muted">Đang tải…</div>

  if (setupNeeded && !user) {
    return <SetupScreen onDone={onAuthed} />
  }
  if (!user) {
    return <LoginScreen onDone={onAuthed} />
  }

  return (
    <div className="page">
      <div className="brand">
        <h1>Viderse</h1>
        <span className="tag">Kịch bản video từ brief</span>
        <span className="spacer" />
        <span className="muted small">{user.name || user.username}</span>
        <button
          className="ghost"
          onClick={() => {
            api.logout().finally(() => setUser(null))
          }}
        >
          Đăng xuất
        </button>
      </div>
      {error && <div className="error">{error}</div>}
      {view.name === 'runs' ? (
        <RunsScreen onError={setError} onOpenScript={(id) => setView({ name: 'script', id })} />
      ) : (
        <ScriptScreen id={view.id} onBack={() => setView({ name: 'runs' })} />
      )}
    </div>
  )
}

function SetupScreen({ onDone }: { onDone: (u: User) => void }) {
  const [code, setCode] = useState('')
  const [token, setToken] = useState('')
  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [recovery, setRecovery] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const verify = async () => {
    setBusy(true)
    setError('')
    try {
      const res = await api.setupVerify(code)
      setToken(res.setupToken)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Không kiểm được mã')
    } finally {
      setBusy(false)
    }
  }

  const create = async () => {
    setBusy(true)
    setError('')
    try {
      const res = await api.setupAdmin(token, name, username, password)
      setRecovery(res.recoveryCode)
      setTimeout(() => onDone(res.user), 1500)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Không tạo được quản trị viên')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page center-card">
      <div className="card">
        <h2>Thiết lập Viderse</h2>
        <p className="hint">Nhập mã thiết lập in trong terminal khi chạy `viderse serve`.</p>
        {recovery ? (
          <>
            <p className="ok">Đã tạo quản trị viên. Mã khôi phục (lưu lại, chỉ hiện một lần):</p>
            <p className="mono">{recovery}</p>
          </>
        ) : !token ? (
          <>
            <label htmlFor="setup-code">Mã thiết lập</label>
            <input id="setup-code" value={code} onChange={(e) => setCode(e.target.value)} placeholder="XXXX-XXXX" className="mono" />
            {error && <div className="error">{error}</div>}
            <button onClick={verify} disabled={busy || !code.trim()}>
              Tiếp tục
            </button>
          </>
        ) : (
          <>
            <label htmlFor="setup-name">Tên hiển thị</label>
            <input id="setup-name" value={name} onChange={(e) => setName(e.target.value)} />
            <label htmlFor="setup-username">Tên đăng nhập</label>
            <input id="setup-username" value={username} onChange={(e) => setUsername(e.target.value)} className="mono" />
            <label htmlFor="setup-password">Mật khẩu</label>
            <input id="setup-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
            {error && <div className="error">{error}</div>}
            <button onClick={create} disabled={busy || !name.trim() || !username.trim() || !password}>
              Tạo quản trị viên
            </button>
          </>
        )}
      </div>
    </div>
  )
}

function LoginScreen({ onDone }: { onDone: (u: User) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      const res = await api.login(username, password)
      onDone(res.user)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Đăng nhập thất bại')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page center-card">
      <div className="card">
        <h2>Đăng nhập Viderse</h2>
        <label htmlFor="login-username">Tên đăng nhập</label>
        <input id="login-username" value={username} onChange={(e) => setUsername(e.target.value)} autoFocus className="mono" />
        <label htmlFor="login-password">Mật khẩu</label>
        <input id="login-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && submit()} />
        {error && <div className="error">{error}</div>}
        <button onClick={submit} disabled={busy || !username.trim() || !password}>
          Đăng nhập
        </button>
      </div>
    </div>
  )
}

function RunsScreen({ onError, onOpenScript }: { onError: (m: string) => void; onOpenScript: (id: string) => void }) {
  const [runs, setRuns] = useState<Run[]>([])
  const [scripts, setScripts] = useState<ScriptSummary[]>([])
  const timer = useRef<number | null>(null)

  const refresh = useCallback(async () => {
    try {
      const [r, s] = await Promise.all([api.listRuns(), api.listScripts()])
      setRuns(r.runs)
      setScripts(s.scripts)
      onError('')
      if (hasActiveRun(r.runs)) {
        timer.current = window.setTimeout(refresh, 2000)
      }
    } catch (e) {
      onError(e instanceof ApiError ? e.message : 'Không tải được danh sách')
      timer.current = window.setTimeout(refresh, 5000)
    }
  }, [onError])

  useEffect(() => {
    refresh()
    return () => window.clearTimeout(timer.current ?? undefined)
  }, [refresh])

  const scriptForRun = (run: Run) => scripts.find((s) => s.run_id === run.id)

  return (
    <>
      <BriefForm onSubmitted={refresh} onError={onError} />
      <div className="card">
        <h2>Lần chạy</h2>
        {runs.length === 0 ? (
          <p className="hint">Chưa có lần chạy nào — gửi brief ở trên.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Brief</th>
                <th>Trạng thái</th>
                <th>Model</th>
                <th>Token</th>
                <th>Chi phí</th>
                <th>Bắt đầu</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {runs.map((run) => {
                const sc = scriptForRun(run)
                return (
                  <tr key={run.id} className={sc ? 'clickable' : ''} onClick={() => sc && onOpenScript(sc.id)}>
                    <td>{briefSummary(run.brief)}</td>
                    <td>
                      <span className={`status ${run.status}`}>{statusLabel(run.status)}</span>
                      {run.error && <div className="error small">{run.error}</div>}
                    </td>
                    <td className="muted small">
                      {run.provider}/{run.model}
                    </td>
                    <td className="muted small">
                      {run.input_tokens}/{run.output_tokens}
                    </td>
                    <td className="muted small">{formatCost(run.cost_usd)}</td>
                    <td className="muted small">{formatTime(run.started_at ?? run.created_at)}</td>
                    <td>{sc && <span className="small">Xem kịch bản →</span>}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>
    </>
  )
}

function BriefForm({ onSubmitted, onError }: { onSubmitted: () => void; onError: (m: string) => void }) {
  const [topic, setTopic] = useState('')
  const [audience, setAudience] = useState('')
  const [lengthKind, setLengthKind] = useState<'seconds' | 'words'>('seconds')
  const [lengthValue, setLengthValue] = useState('60')
  const [wpm, setWpm] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const submit = async () => {
    const { brief, error } = validateBrief({ topic, audience, lengthKind, lengthValue, wpm })
    if (error || !brief) {
      setError(error ?? 'Brief chưa hợp lệ')
      return
    }
    setBusy(true)
    setError('')
    try {
      await api.createRun(brief)
      onSubmitted()
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : 'Không gửi được brief'
      setError(msg)
      onError('')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="card">
      <h2>Brief mới</h2>
      <p className="hint">Writer agent viết kịch bản video từ brief — kết quả lưu vào bảng scripts.</p>
      <label htmlFor="brief-topic">Chủ đề</label>
      <textarea id="brief-topic" value={topic} onChange={(e) => setTopic(e.target.value)} placeholder="Vd: Vì sao đế chế La Mã sụp đổ" />
      <label htmlFor="brief-audience">Khán giả</label>
      <input id="brief-audience" value={audience} onChange={(e) => setAudience(e.target.value)} placeholder="Vd: người thích lịch sử, xem khi nấu ăn" />
      <div className="row">
        <div>
          <label htmlFor="brief-length">Độ dài mục tiêu</label>
          <input id="brief-length" value={lengthValue} onChange={(e) => setLengthValue(e.target.value)} inputMode="numeric" />
        </div>
        <div>
          <label htmlFor="brief-unit">Đơn vị</label>
          <select id="brief-unit" value={lengthKind} onChange={(e) => setLengthKind(e.target.value as 'seconds' | 'words')}>
            <option value="seconds">giây</option>
            <option value="words">từ</option>
          </select>
        </div>
        <div>
          <label htmlFor="brief-wpm">Tốc độ đọc (từ/phút, trống = 150)</label>
          <input id="brief-wpm" value={wpm} onChange={(e) => setWpm(e.target.value)} inputMode="numeric" />
        </div>
      </div>
      {error && <div className="error">{error}</div>}
      <button onClick={submit} disabled={busy}>
        Gửi brief
      </button>
    </div>
  )
}

function ScriptScreen({ id, onBack }: { id: string; onBack: () => void }) {
  const [script, setScript] = useState<ScriptSummary | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .getScript(id)
      .then((r) => setScript(r.script))
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Không tải được kịch bản'))
  }, [id])

  if (error) {
    return (
      <div className="card">
        <div className="error">{error}</div>
        <button className="ghost" onClick={onBack}>
          ← Quay lại
        </button>
      </div>
    )
  }
  if (!script) return <div className="muted">Đang tải…</div>

  return (
    <div className="card">
      <button className="ghost" onClick={onBack}>
        ← Danh sách
      </button>
      <h2>{script.title || '(không tiêu đề)'}</h2>
      <p className="hint">
        {script.word_count} từ · ~{formatDuration(script.est_duration_sec)} · {formatTime(script.created_at)}
      </p>
      {(script.sections ?? []).map((sec, i) => (
        <div className="section" key={i}>
          <span className="kind">{sec.kind}</span>
          <span className="badges">
            {sec.rehook && 're-hook '}
            {sec.cta && 'cta'}
          </span>
          <h4>{sec.title}</h4>
          {sec.scenes.length > 0 ? (
            sec.scenes.map((scene, j) => (
              <div className="scene" key={j}>
                <div className="narration">{scene.narration}</div>
                {scene.visual_note && <div className="visual">🎬 {scene.visual_note}</div>}
              </div>
            ))
          ) : (
            <div className="narration">{sec.narration}</div>
          )}
        </div>
      ))}
      <details>
        <summary className="muted small">Toàn văn</summary>
        <div className="narration">{script.full_text}</div>
      </details>
    </div>
  )
}
