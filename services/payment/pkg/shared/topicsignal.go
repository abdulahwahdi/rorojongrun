package shared

import "sync"

// The callback consumer reloads its topic list from the DB on an interval; this signal
// lets a change made through this instance's admin API apply immediately. Other replicas
// pick the change up on their next reload.
var (
	topicSignalMu sync.Mutex
	topicSignal   = make(chan struct{}, 1)
)

// NotifyTopicsChanged asks the callback consumer to reload its topics now
func NotifyTopicsChanged() {
	topicSignalMu.Lock()
	defer topicSignalMu.Unlock()
	select {
	case topicSignal <- struct{}{}:
	default: // a reload is already pending
	}
}

// TopicsChanged is signalled by NotifyTopicsChanged
func TopicsChanged() <-chan struct{} { return topicSignal }
