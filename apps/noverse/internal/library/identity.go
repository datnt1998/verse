package library

import "github.com/datnt1998/verse/platform/identity"

// IdentityOptions 提供本应用的 project 空间规则给 platform/identity：
// 项目 ID 即书库目录 slug（ValidProjectID），无前缀 ID 归匿名 legacy 项目。
// serve/CLI/测试构建账号 Store 时统一取这里，避免散落同一份字面量。
func IdentityOptions() identity.Options {
	return identity.Options{ValidProjectID: ValidProjectID, LegacyProjectID: LegacyProjectID}
}
