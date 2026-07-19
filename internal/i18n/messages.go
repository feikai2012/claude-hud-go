package i18n

// Message catalogs, ported verbatim from src/i18n/{en,zh-Hans,zh-Hant}.ts.

var en = map[string]string{
	"label.context":       "Context",
	"label.usage":         "Usage",
	"label.weekly":        "Weekly",
	"label.approxRam":     "Approx RAM",
	"label.promptCache":   "Cache",
	"label.rules":         "rules",
	"label.hooks":         "hooks",
	"label.estimatedCost": "Est.",
	"label.cost":          "Cost",
	"label.tokens":        "Tokens",
	"label.sessionStarted": "Started",
	"label.lastReply":     "Last reply",
	"label.advisor":       "Advisor",
	"label.compactions":   "Compactions",

	"status.limitReached":     "Limit reached",
	"status.allTodosComplete": "All todos complete",
	"status.expired":          "expired",

	"format.resets":       "resets",
	"format.resetsIn":     "resets in",
	"format.absoluteTime": "at {time}",
	"format.in":           "in",
	"format.cache":        "cache",
	"format.out":          "out",
	"format.tok":          "tok",
	"format.tokPerSec":    "tok/s",
	"format.justNow":      "just now",
	"format.relativeTime": "{value} ago",

	"init.initializing": "[claude-hud] Initializing...",
	"init.macosNote":    "[claude-hud] Note: On macOS, you may need to restart Claude Code for the HUD to appear.",
}

var zhHans = map[string]string{
	"label.context":       "上下文",
	"label.usage":         "用量",
	"label.weekly":        "本周",
	"label.approxRam":     "内存",
	"label.promptCache":   "缓存",
	"label.rules":         "规则",
	"label.hooks":         "钩子",
	"label.estimatedCost": "估算",
	"label.cost":          "费用",
	"label.tokens":        "词元",
	"label.sessionStarted": "开始",
	"label.lastReply":     "上次回复",
	"label.advisor":       "顾问",
	"label.compactions":   "压缩次数",

	"status.limitReached":     "已达上限",
	"status.allTodosComplete": "全部完成",
	"status.expired":          "已过期",

	"format.resets":       "重置于",
	"format.resetsIn":     "重置剩余",
	"format.absoluteTime": "{time}",
	"format.in":           "输入",
	"format.cache":        "缓存",
	"format.out":          "输出",
	"format.tok":          "词元",
	"format.tokPerSec":    "tok/s",
	"format.justNow":      "刚刚",
	"format.relativeTime": "{value} 前",

	"init.initializing": "[claude-hud] 正在初始化...",
	"init.macosNote":    "[claude-hud] 注意：在 macOS 上，您可能需要重启 Claude Code 才能显示 HUD。",
}

var zhHant = map[string]string{
	"label.context":       "上下文",
	"label.usage":         "用量",
	"label.weekly":        "本週",
	"label.approxRam":     "記憶體",
	"label.promptCache":   "快取",
	"label.rules":         "規則",
	"label.hooks":         "Hook",
	"label.estimatedCost": "估算",
	"label.cost":          "費用",
	"label.tokens":        "Token",
	"label.sessionStarted": "開始",
	"label.lastReply":     "上次回覆",
	"label.advisor":       "顧問",
	"label.compactions":   "壓縮次數",

	"status.limitReached":     "已達上限",
	"status.allTodosComplete": "全部完成",
	"status.expired":          "已過期",

	"format.resets":       "重置於",
	"format.resetsIn":     "重置剩餘",
	"format.absoluteTime": "{time}",
	"format.in":           "輸入",
	"format.cache":        "快取",
	"format.out":          "輸出",
	"format.tok":          "tok",
	"format.tokPerSec":    "tok/s",
	"format.justNow":      "剛剛",
	"format.relativeTime": "{value} 前",

	"init.initializing": "[claude-hud] 正在初始化...",
	"init.macosNote":    "[claude-hud] 注意：在 macOS 上，您可能需要重新啟動 Claude Code 才能顯示 HUD。",
}
