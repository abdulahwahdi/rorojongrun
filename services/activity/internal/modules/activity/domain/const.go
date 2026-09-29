package domain

// TaskNameActivity is the task queue worker task name used to enqueue (from the
// usecase) and mount (from the worker handler) the async save-activity-log job.
// Keep these in sync — a mismatch here means enqueued jobs are never picked up.
const TaskNameActivity = "activity-task"
