package auction

import (
	"fmt"
	"strings"
)

const FantasyPoolSize = 543

var defaultCategories = []string{"MARQUEE", "CAPPED_BATTER", "CAPPED_BOWLER", "CAPPED_ALL_ROUNDER", "CAPPED_WICKETKEEPER", "UNCAPPED_BATTER", "UNCAPPED_BOWLER", "UNCAPPED_ALL_ROUNDER", "UNCAPPED_WICKETKEEPER", "ASSOCIATE_ALL_ROUNDER"}

// QueueOptions permits a complete category permutation and optional category-local shuffles.
type QueueOptions struct {
	CategoryOrder     []string `json:"category_order"`
	ShuffleCategories []string `json:"shuffle_categories"`
}

func ValidateQueueOptions(in QueueOptions) (QueueOptions, error) {
	out := QueueOptions{CategoryOrder: append([]string{}, in.CategoryOrder...), ShuffleCategories: append([]string{}, in.ShuffleCategories...)}
	if len(out.CategoryOrder) == 0 {
		out.CategoryOrder = append([]string{}, defaultCategories...)
	}
	allowed := map[string]bool{}
	for _, c := range defaultCategories {
		allowed[c] = true
	}
	if len(out.CategoryOrder) != len(allowed) {
		return QueueOptions{}, fmt.Errorf("category_order must contain all %d categories exactly once", len(allowed))
	}
	for _, list := range [][]string{out.CategoryOrder, out.ShuffleCategories} {
		seen := map[string]bool{}
		for i, c := range list {
			c = strings.ToUpper(strings.TrimSpace(c))
			if !allowed[c] || seen[c] {
				return QueueOptions{}, fmt.Errorf("unknown or duplicate category: %s", c)
			}
			seen[c] = true
			list[i] = c
		}
	}
	return out, nil
}
