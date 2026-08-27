package model

// ModelCatalogEntry describes a model exposed by the platform's gateway.
// Prices are display-only catalog metadata; actual billing prices come from
// the model_prices table (admin-configurable, CNY per 1M tokens).
type ModelCatalogEntry struct {
	ID              string   `json:"id"`                // model id used in API calls
	Provider        string   `json:"provider"`          // openai | anthropic
	Name            string   `json:"name"`              // display name
	Description     string   `json:"description"`       // capability summary
	Context         string   `json:"context"`           // context window, e.g. "128K"
	InputPrice      string   `json:"input_price"`       // CNY / 1M input tokens
	OutputPrice     string   `json:"output_price"`      // CNY / 1M output tokens
	CacheReadPrice  string   `json:"cache_read_price"`  // CNY / 1M cached-input tokens
	CacheWritePrice string   `json:"cache_write_price"` // CNY / 1M cache-write tokens
	SupportUnlimited bool    `json:"support_unlimited"`  // 模型是否支持无限火力活动
	UnlimitedEnabled bool    `json:"unlimited_enabled"`  // 无限火力活动是否开启
	Features        []string `json:"features"`          // capability tags
	Status          string   `json:"status"`            // available
}

// GetModelCatalog returns the built-in catalog of available models.
func GetModelCatalog() []ModelCatalogEntry {
	return []ModelCatalogEntry{
		{
			ID:          "gpt-4o",
			Provider:    "openai",
			Name:        "GPT-4o",
			Description: "OpenAI 旗舰多模态模型，文本与视觉理解均衡，适合通用对话、工具调用与复杂推理",
			Context:     "128K",
			InputPrice:  "$2.50", OutputPrice: "$10.00",
			Features: []string{"多模态", "流式输出", "函数调用", "JSON 输出"},
			Status:   "available",
		},
		{
			ID:          "gpt-4o-mini",
			Provider:    "openai",
			Name:        "GPT-4o mini",
			Description: "高性价比轻量模型，速度与成本俱佳，适合大规模量级调用与实时场景",
			Context:     "128K",
			InputPrice:  "$0.15", OutputPrice: "$0.60",
			Features: []string{"多模态", "流式输出", "函数调用"},
			Status:   "available",
		},
		{
			ID:          "gpt-4",
			Provider:    "openai",
			Name:        "GPT-4",
			Description: "经典高精度模型，适合高要求文本生成与复杂指令跟随",
			Context:     "8K",
			InputPrice:  "$30.00", OutputPrice: "$60.00",
			Features: []string{"高质量文本", "函数调用"},
			Status:   "available",
		},
		{
			ID:          "gpt-3.5-turbo",
			Provider:    "openai",
			Name:        "GPT-3.5 Turbo",
			Description: "成熟稳定的大众模型，快速响应，适合多数通用文本任务",
			Context:     "16K",
			InputPrice:  "$0.0015", OutputPrice: "$0.002",
			Features: []string{"低成本", "流式输出", "函数调用"},
			Status:   "available",
		},
		{
			ID:          "claude-3-5-sonnet",
			Provider:    "anthropic",
			Name:        "Claude 3.5 Sonnet",
			Description: "Anthropic 最高性价比智能模型，代码、长文与推理表现优秀",
			Context:     "200K",
			InputPrice:  "$3.00", OutputPrice: "$15.00",
			Features: []string{"长上下文", "流式输出", "工具调用", "代码能力"},
			Status:   "available",
		},
		{
			ID:          "claude-3-sonnet",
			Provider:    "anthropic",
			Name:        "Claude 3 Sonnet",
			Description: "均衡型旗舰模型，兼具智能与速度，适合生产环境任务",
			Context:     "200K",
			InputPrice:  "$3.00", OutputPrice: "$15.00",
			Features: []string{"长上下文", "流式输出", "工具调用"},
			Status:   "available",
		},
		{
			ID:          "claude-3-haiku",
			Provider:    "anthropic",
			Name:        "Claude 3 Haiku",
			Description: "秒级响应的轻量模型，适合高并发、低延迟的规模化场景",
			Context:     "200K",
			InputPrice:  "$0.25", OutputPrice: "$1.25",
			Features: []string{"低延迟", "流式输出", "长上下文"},
			Status:   "available",
		},
		{
			ID:          "claude-3-opus",
			Provider:    "anthropic",
			Name:        "Claude 3 Opus",
			Description: "顶级智能模型，面向最复杂推理与高质量创作场景",
			Context:     "200K",
			InputPrice:  "$15.00", OutputPrice: "$75.00",
			Features: []string{"最强推理", "长上下文", "流式输出"},
			Status:   "available",
		},
	}
}
