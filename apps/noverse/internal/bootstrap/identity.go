package bootstrap

import "github.com/datnt1998/verse/core/auth"

// core/auth 是跨应用共享包：credential 存储路径、OAuth 回调页品牌、错误里的
// 登录命令、Claude Code wire version 的环境变量都来自 auth.Options。本二进制
// 的身份必须在任何包打开 credential 库之前装好，因此放在 init()：凡是使用
// core/auth 的 Noverse 包（cmd、tui、web、host）都已 import bootstrap，main
// 运行前 Go 保证本 init 已执行。
func init() {
	auth.Configure(auth.Options{
		AppName:              "Noverse",
		Purpose:              "để viết truyện",
		LoginCommand:         "ainovel-cli login",
		StoreDir:             configDirName,
		ClaudeCodeVersionEnv: "AINOVEL_CLAUDE_CODE_VERSION",
	})
}
