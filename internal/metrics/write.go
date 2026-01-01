package metrics

import (
	"strings"
	"time"
)

// Setup metrics map for this collection interval
func (registry *Registry) NewTimeSlice(now time.Time, interval time.Duration) (timeSlice time.Time) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if interval <= 0 {
		timeSlice = time.Now()
	}

	// Round down for this interval
	timeSlice = now.Truncate(interval)
	if registry.metrics[timeSlice] == nil {
		registry.metrics[timeSlice] = make(map[string]map[string]Metric)
	}
	return
}

// Adds batch of metrics to a time slice
func (registry *Registry) Add(timeSlice time.Time, metrics []Metric) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if registry.metrics[timeSlice] == nil {
		return
	}

	for _, metric := range metrics {
		namespace := strings.Join(metric.Namespace, "/")

		// Ensure namespace map is initialized
		if registry.metrics[timeSlice][namespace] == nil {
			registry.metrics[timeSlice][namespace] = make(map[string]Metric)
		}

		// Write metric to map
		registry.metrics[timeSlice][namespace][metric.Name] = metric
	}
}
