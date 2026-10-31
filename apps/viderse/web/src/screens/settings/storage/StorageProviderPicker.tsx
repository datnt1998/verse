import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import type { S3Provider } from '../../../api/storageServer'
import { serverStorageCopy as t } from '../../../i18n/storageServer'
import { styles as s } from './serverStorage.styles'

export function StorageProviderPicker({ current, onPick }: { current?: S3Provider; onPick: (provider: S3Provider) => void }) {
  return <div {...stylex.props(s.grid)} role="list" aria-label={t.pick}>
    {(Object.keys(t.providers) as S3Provider[]).map(id => {
      const provider = t.providers[id]
      return <button key={id} type="button" {...stylex.props(s.picker)} aria-current={current === id ? 'true' : undefined} onClick={() => onPick(id)}>
        <span {...stylex.props(s.logo)} aria-hidden="true">{provider.initials}</span>
        <span {...stylex.props(s.grow, s.column)}><span><b>{provider.name}</b> {current === id ? <small {...stylex.props(s.good)}>{t.active}</small> : null}</span><span {...stylex.props(s.hint)}>{provider.hint}</span><span {...stylex.props(s.hint, s.muted)}>{id === 'garage' ? t.recommendProxy : t.recommendDirect}</span></span>
        <Icon name="arrow-r" size={14} />
      </button>
    })}
  </div>
}
