package metrics

import "time"

// Deletes metrics in registry older than max allowed metric age based on supplied current time
func (registry *Registry) Prune(currentTime time.Time, maxAge time.Duration) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for timeSlice := range registry.metrics {
		// time slice key is older than allowed maximum age
		if currentTime.Sub(timeSlice) > maxAge {
			delete(registry.metrics, timeSlice)
		}
	}
}
