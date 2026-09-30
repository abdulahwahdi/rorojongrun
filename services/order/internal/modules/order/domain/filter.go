package domain

import shareddomain "monorepo/services/order/pkg/shared/domain"

// FilterSummary is the bookkeeping report filter: the order filter plus a grouping
type FilterSummary struct {
	shareddomain.FilterOrder
	GroupBy string `json:"groupBy,omitempty"`
	// Timezone the day grouping uses, the merchant's (or the default merchant's) timezone
	Timezone string `json:"-"`
}
