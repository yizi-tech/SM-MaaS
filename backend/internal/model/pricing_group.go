package model

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// PricingGroup defines a billing multiplier applied on top of the per-model
// price from the model_prices table. Pay-per-use cost = table price * multiplier;
// subscription credits are debited as tokens * multiplier.
// A model is served by the first enabled group (ordered by id) whose model
// list matches; if no group matches, the multiplier defaults to 1.
type PricingGroup struct {
	ID         uint            `gorm:"primaryKey" json:"id"`
	Name       string          `gorm:"size:50;not null" json:"name"`
	Multiplier decimal.Decimal `gorm:"type:decimal(10,4);not null;default:1" json:"multiplier"`
	Models     StringSlice     `gorm:"type:text" json:"models"` // empty = all models; exact name or prefix wildcard "gpt-4o*"
	Enabled    bool            `gorm:"default:true" json:"enabled"`
	Remark     string          `gorm:"size:200" json:"remark"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// MatchesModel reports whether the group applies to the given model name.
// An empty model list means the group matches every model.
func (g *PricingGroup) MatchesModel(modelName string) bool {
	if len(g.Models) == 0 {
		return true
	}
	modelName = strings.TrimSpace(modelName)
	for _, m := range g.Models {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if strings.HasSuffix(m, "*") {
			if strings.HasPrefix(modelName, strings.TrimSuffix(m, "*")) {
				return true
			}
		} else if m == modelName {
			return true
		}
	}
	return false
}