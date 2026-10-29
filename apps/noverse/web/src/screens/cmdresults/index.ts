// screens/cmdresults — CmdResults.dc.html (14-jobs-dock §2): bảng /model, /diag và thẻ kết quả
// lệnh trong nhật ký Workspace.
export { ModelPanel } from './ModelPanel'
export { DiagPanel } from './DiagPanel'
export {
  ExportDoneCard, ExportExistsCard, ExportRunCard, JobFailedCard, SimulateDoneCard, SimulateRunCard,
  SyncCheckCard, SyncDoneCard, SyncRunCard,
} from './cards'
export { CmdChip } from './parts'
export { CommandResults, useCommandResults, type CmdItem, type CommandResultsCtl } from './results'
