// Central registry for storing time-based metrics and their associated data
package metrics

import "time"

// Creates new metric registry storage
func New() (new *Registry) {
	new = &Registry{
		metrics: make(map[time.Time]map[string]map[string]Metric),
	}
	return
}
