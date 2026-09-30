package domain

// Summary groupings
const (
	GroupByDay     = "day"
	GroupByMethod  = "method"
	GroupByOutlet  = "outlet"
	GroupByCashier = "cashier"
)

// Cron job names
const (
	JobFlushOutbox = "order-flush-outbox"
	// the export module's background jobs run in this module's cron (the only scheduler of the service)
	JobSweepExports = "order-export-sweep"
	JobPurgeExports = "order-export-purge"
)
