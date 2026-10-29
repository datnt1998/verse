// “Cách dùng lệnh” — Commands.dc.html trạng thái intro (cột giữa).
import type { CommandSpec } from '../../api/types'
import { orderedSpecs } from '../../commands/model'
import { CMD_COLOR } from '../../commands/tokens'
import { CMD_UI, copyOf } from '../../i18n/commands.vi'
import { Icon } from '@noverse/ui'
import { Kbd } from '@noverse/ui'

export function CommandIntro({ specs, onPick }: { specs: CommandSpec[]; onPick: (name: string) => void }) {
  const ordered = orderedSpecs(specs)
  const auto = ordered.filter((spec) => spec.autoExecute)
  const manual = ordered.filter((spec) => !spec.autoExecute)

  return (
    <article className="rise" style={{
      maxWidth: 820, padding: '28px 36px 48px', display: 'flex', flexDirection: 'column', gap: 22,
    }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        <span className="lbl">{CMD_UI.introEyebrow}</span>
        <h2 style={{ margin: 0, fontSize: 26, fontWeight: 600, letterSpacing: '-0.02em' }}>{CMD_UI.introTitle}</h2>
        <p style={{ margin: 0, fontSize: 14.5, color: 'var(--color-fg-3)', maxWidth: 640 }}>
          {CMD_UI.introLeadBefore}
          <span className="mono" style={{ color: 'var(--cmd-name)' }}>{CMD_UI.introLeadSlash}</span>
          {CMD_UI.introLeadAfter}
        </p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 16 }}>
        <div className="card" aria-hidden="true" style={{
          borderRadius: 16, padding: 16, display: 'flex', flexDirection: 'column', justifyContent: 'flex-end',
          gap: 8, minHeight: 236,
          background: 'radial-gradient(120% 80% at 50% 0%, rgba(201,166,245,0.06), transparent 60%), #121214',
        }}>
          <div style={{
            borderRadius: 12, background: 'var(--color-surface-3)', border: '1px solid rgba(255,255,255,0.1)',
            padding: 6, display: 'flex', flexDirection: 'column', gap: 2, boxShadow: '0 18px 40px -16px rgba(0,0,0,0.8)',
          }}>
            <div style={{
              display: 'flex', alignItems: 'center', gap: 10, padding: '7px 8px', borderRadius: 8, background: '#26262A',
            }}>
              <span className="mono" style={{ fontSize: 12.5, color: 'var(--cmd-name)', width: 64 }}>/export</span>
              <span style={{
                flex: 1, fontSize: 12, color: 'var(--color-fg-2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
              }}>{copyOf('export')?.title}</span>
              <Kbd>Tab</Kbd>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '7px 8px', borderRadius: 8 }}>
              <span className="mono" style={{ fontSize: 12.5, color: 'var(--color-fg-3)', width: 64 }}>/diag</span>
              <span style={{
                flex: 1, fontSize: 12, color: 'var(--color-fg-4)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
              }}>{copyOf('diag')?.title}</span>
            </div>
          </div>
          <div style={{
            height: 44, borderRadius: 12, background: 'var(--color-surface-0)',
            border: '1px solid rgba(201,166,245,0.45)', display: 'flex', alignItems: 'center', padding: '0 12px', gap: 8,
          }}>
            <span className="mono" style={{ fontSize: 13.5, color: 'var(--color-fg)' }}>/ex<span className="cmd-caret" /></span>
          </div>
          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', fontSize: 11.5, color: 'var(--color-fg-4)' }}>
            <span><Kbd>↑</Kbd> <Kbd>↓</Kbd> {CMD_UI.keysHint.pick}</span>
            <span><Kbd>Tab</Kbd> {CMD_UI.keysHint.fill}</span>
            <span><Kbd>Enter</Kbd> {CMD_UI.enterRun}</span>
            <span><Kbd>Esc</Kbd> {CMD_UI.keysHint.close}</span>
          </div>
        </div>

        <ol style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 10 }}>
          {CMD_UI.introSteps.map((step, index) => (
            <li key={step.title} style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
              <span className="num" style={{
                width: 24, height: 24, borderRadius: '50%', border: '1px solid rgba(255,255,255,0.14)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 12,
                color: 'var(--color-fg-2)', flex: '0 0 auto', marginTop: 1,
              }}>{index + 1}</span>
              <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <span style={{ fontSize: 13.5, fontWeight: 500 }}>{step.title}</span>
                <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{step.text}</span>
              </span>
            </li>
          ))}
        </ol>
      </div>

      <section className="card" aria-labelledby="h-syntax" style={{
        borderRadius: 16, padding: '18px 20px', display: 'flex', flexDirection: 'column', gap: 12,
      }}>
        <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
          <h3 id="h-syntax" style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{CMD_UI.syntaxTitle}</h3>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{CMD_UI.syntaxNote}</span>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 8 }}>
          {CMD_UI.legend.map((row) => (
            <div key={row.tok} style={{
              display: 'flex', gap: 12, alignItems: 'flex-start', padding: '10px 12px', borderRadius: 10,
              background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.05)',
            }}>
              <span className="mono" style={{ flex: '0 0 104px', fontSize: 13, color: CMD_COLOR[row.tone], whiteSpace: 'nowrap' }}>{row.tok}</span>
              <span style={{ minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
                <span style={{ fontSize: 13 }}>{row.means}</span>
                <span className="mono" style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{row.ex}</span>
              </span>
            </div>
          ))}
        </div>
      </section>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 16 }}>
        {[
          { kind: CMD_UI.kindAuto, specs: auto, icon: 'flash', color: 'var(--cmd-name)' },
          { kind: CMD_UI.kindManual, specs: manual, icon: 'edit-2', color: 'var(--cmd-value)' },
        ].map(({ kind, specs: list, icon, color }) => (
          <section key={kind.title} className="card" style={{
            borderRadius: 16, padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 10,
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <Icon name={icon} size={17} style={{ color }} />
              <h3 style={{ margin: 0, fontSize: 14, fontWeight: 600 }}>{kind.title}</h3>
            </div>
            <p style={{ margin: 0, fontSize: 12.5, color: 'var(--color-fg-3)' }}>{kind.text}</p>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {list.map((spec) => (
                <button key={spec.name} type="button" onClick={() => onPick(spec.name)} className="help-chip mono" style={{
                  height: 26, padding: '0 9px', borderRadius: 7, border: '1px solid var(--line-4)',
                  background: 'var(--color-surface-0)', color: 'var(--color-fg-2)', fontSize: 12, whiteSpace: 'nowrap',
                }}>/{spec.name}</button>
              ))}
            </div>
          </section>
        ))}
      </div>

      <section className="card" aria-labelledby="h-deny" style={{
        borderRadius: 16, padding: '18px 20px', display: 'flex', flexDirection: 'column', gap: 4,
      }}>
        <h3 id="h-deny" style={{ margin: '0 0 8px', fontSize: 15, fontWeight: 600 }}>{CMD_UI.deniesTitle}</h3>
        {CMD_UI.denies.map((row, index) => (
          <div key={row.title} style={{
            display: 'grid', gridTemplateColumns: 'minmax(0, 1.1fr) minmax(0, 1fr)', gap: 16, padding: '10px 0',
            borderTop: index ? '1px solid rgba(255,255,255,0.06)' : 'none',
          }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              <span style={{ fontSize: 13, fontWeight: 500 }}>{row.title}</span>
              <span className="mono" style={{ fontSize: 11.5, color: 'var(--cmd-error-fg)' }}>{row.msg}</span>
            </div>
            <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{row.fix}</span>
          </div>
        ))}
      </section>
    </article>
  )
}
