package model

import (
	"strings"
	"time"
)

// LLMChannel represents an upstream LLM provider channel managed by admins.
// The gateway routes a request to the highest-priority enabled channel whose
// model list matches the requested model.
type LLMChannel struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	Name      string      `gorm:"size:100;not null" json:"name"`
	Type      string      `gorm:"size:20;not null;default:openai" json:"type"` // openai | anthropic
	BaseURL   string      `gorm:"size:500" json:"base_url"`
	APIKey    string      `gorm:"size:500" json:"api_key"`
	Models    StringSlice `gorm:"type:text" json:"models"` // empty = all models; supports exact names or prefix wildcard "gpt-4o*"
	Priority  int         `gorm:"default:0" json:"priority"` // higher value wins
	Enabled   bool        `gorm:"default:false" json:"enabled"`
	Remark    string      `gorm:"size:500" json:"remark"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// MatchesModel reports whether the channel serves the given model name.
// An empty model list means the channel accepts all models.
func (c *LLMChannel) MatchesModel(model string) bool {
	if len(c.Models) == 0 {
		return true
	}
	model = strings.TrimSpace(model)
	for _, m := range c.Models {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if strings.HasSuffix(m, "*") {
			if strings.HasPrefix(model, strings.TrimSuffix(m, "*")) {
				return true
			}
		} else if m == model {
			return true
		}
	}
	return false
}