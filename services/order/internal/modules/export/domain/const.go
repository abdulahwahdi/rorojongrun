package domain

import "time"

// TaskExport is the task queue task that generates an export; its argument is the job id
const TaskExport = "order-export"

// Worker policy
const (
	// MaxAttempts runs of a job before it is failed
	MaxAttempts = 3
	// BatchSize rows are read per query (keyset paging)
	BatchSize = 1000
	// QueuedGrace is how long a queued job may wait before the sweeper queues it again
	QueuedGrace = time.Minute
	// HeartbeatTimeout is how long a running job may go silent before the sweeper takes it over
	HeartbeatTimeout = 5 * time.Minute
)
