// Account dialogs: QR pairing and personal devices; member administration lives in Settings.
import { setDialog } from '../../router'
import { DevicesDialog } from './DevicesDialog'
import { PairingDialog } from './PairingDialog'

const close = () => setDialog(null)

/** Account overlays on the current route. */
export function AccountDialogs({ dialog }: { dialog: 'qr' | 'devices' }) {
  if (dialog === 'qr') return <PairingDialog onClose={close} />
  return <DevicesDialog onClose={close} />
}
