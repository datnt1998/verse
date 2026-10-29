// src/ui giờ chỉ còn phần gắn app (i18n/data/router) — primitive/tokens/icons ở gói @noverse/ui.
// Import trực tiếp từ './<file>' hoặc '@noverse/ui'; thùng này gom nốt API app-side thường dùng.
export { CharAvatar, KindChip, KindLegend, badgeGeometry, type CharAvatarProps } from './CharAvatar'
export { ModelContextChoices } from './ModelContextChoices'
export { PanelRefreshBar } from './PanelRefreshBar'
export { ConnectionBanner } from './ConnectionBanner'
export {
  confirmAllDevicesLogout, confirmDeleteAccount, confirmDeleteStory, confirmDeviceLogout,
  confirmOverwrite, confirmPause, confirmUnsavedLeave, confirmationPreset,
  CONFIRM_SITUATIONS, type ConfirmSituation,
} from './confirmPresets'
export { useUnsavedGuard } from './useUnsavedGuard'
export { AGENT_PALETTE, agentAura, agentAuraBg, agentBar, agentChipBg, agentKindOf, agentOrbGlow } from './agentPalette'
