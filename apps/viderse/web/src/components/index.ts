// Bộ phận dùng lại của Viderse — “mỗi thứ một nơi”: ánh xạ trợ lý, logo,
// avatar kênh, nhãn trạng thái, nút, hộp xác nhận, trạng thái hệ thống.
// Chip/Banner/Toast/menu/select… dùng trực tiếp từ @noverse/ui (kit) — chỉ bọc
// khi design đặt tên hoặc hành vi riêng.
export { ASSISTANTS, ASSISTANT_INK, ASSISTANT_ORB, assistantName, type Assistant } from './agents'
export { Orb } from './Orb'
export { LensLogo } from './LensLogo'
export { ChannelAvatar } from './ChannelAvatar'
export { StatusLabel, statusColor, statusName, type VideoStatus } from './StatusLabel'
export { ActionButton, type ActionVariant, type ActionButtonProps } from './ActionButton'
export { SystemLoading, SystemNotFound, EmptyWorkbench } from './states'
export {
  ConfirmDialog,
  confirmPause, confirmOverwrite, confirmProviderLogout, confirmDeleteVideo,
  confirmDeleteChannel, confirmStopAll, confirmLeaveUnsaved, confirmAutonomy,
} from './confirm'
export { ScreenEmpty } from './ScreenEmpty'
export { DevelopEffect, type DevelopEffectProps, type DevelopEffectState } from './DevelopEffect'
export { ResponsiveSelect, ResponsiveChipSelect, ResponsiveActionMenu } from './responsive-overlays'
