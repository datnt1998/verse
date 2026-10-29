// @noverse/ui/data — lõi lớp dữ liệu dùng chung giữa các app (không có kiểu/khoá/endpoint
// của app nào). App định nghĩa `keys`/bảng event→khoá của riêng mình và tạo client
// SSE bằng `createEventsClient` với `eventTypes` của app đó.
export { createCache, cache, invalidate, setData, getData, sameData } from './cache'
export type { CacheOptions, EntrySnapshot, Fetcher, ResourceCache, ResourceStatus } from './cache'
export { keyHash, matchesPrefix } from './keys'
export type { Key, KeyPart } from './keys'
export { createEventsClient, fetchSource, BACKOFF_MS, STREAM_STALL_MS, STREAM_WAKE_STALL_MS } from './events'
export type { EventHandler, EventsClient, EventsClientOptions, EventsStatus, FetchSourceOptions } from './events'
