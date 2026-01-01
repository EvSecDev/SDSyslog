package metrics

import (
	"sort"
	"strings"
	"time"
)

// Supports exact match or prefix match. Empty query matches all.
func matchesNamespace(metricNS, queryNS []string) (matches bool) {
	if len(queryNS) == 0 {
		matches = true
		return
	}
	if len(metricNS) < len(queryNS) {
		return
	}
	for i := 0; i < len(queryNS); i++ {
		if metricNS[i] != queryNS[i] {
			return
		}
	}
	matches = true
	return
}

// Returns all metrics matching given name and namespace prefix.
// If name is empty, returns all names.
// If namespacePrefix is empty, returns all namespaces.
// Optional: start/end time window filter.
func (registry *Registry) Search(name string, namespacePrefix []string, start, end time.Time) (results []Metric) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	// Collect all timestamps
	var timestamps []time.Time
	for ts := range registry.metrics {
		if !start.IsZero() && ts.Before(start) {
			continue
		}
		if !end.IsZero() && ts.After(end) {
			continue
		}
		timestamps = append(timestamps, ts)
	}

	// Sort timestamps oldest -> newest
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i].Before(timestamps[j])
	})

	// Iterate timestamps in order
	for _, ts := range timestamps {
		nsMap := registry.metrics[ts]
		for nsStr, metricsMap := range nsMap {
			ns := strings.Split(nsStr, "/")
			if !matchesNamespace(ns, namespacePrefix) {
				continue
			}
			for metricName, metric := range metricsMap {
				if name == "" || metricName == name {
					results = append(results, metric)
				}
			}
		}
	}

	return
}

// Finds all metric types that match given search filters (time-independent). Returns all when all filters are empty.
func (registry *Registry) Discover(name, description string, namespacePrefix []string, unit string, metricType MetricType) (results []Metric) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	seen := make(map[string]Metric)

	for _, nsMap := range registry.metrics {
		for nsStr, metricsMap := range nsMap {

			ns := strings.Split(nsStr, "/")
			if !matchesNamespace(ns, namespacePrefix) {
				continue
			}

			for _, metric := range metricsMap {

				// Filters
				if name != "" && !strings.Contains(metric.Name, name) {
					continue
				}
				if description != "" && !strings.Contains(metric.Description, description) {
					continue
				}
				if unit != "" && metric.Value.Unit != unit {
					continue
				}
				if metricType != "" && metric.Type != metricType {
					continue
				}

				// Deduplication key
				key := strings.Join([]string{
					nsStr,
					metric.Name,
					string(metric.Type),
					metric.Value.Unit,
				}, "|")

				if _, exists := seen[key]; exists {
					continue
				}

				// Strip time + raw value
				discovered := Metric{
					Name:        metric.Name,
					Description: metric.Description,
					Namespace:   metric.Namespace,
					Type:        metric.Type,
					Value: MetricValue{
						Unit: metric.Value.Unit,
					},
					// Timestamp intentionally zero
				}

				seen[key] = discovered
			}
		}
	}

	// Stable output
	results = make([]Metric, 0, len(seen))
	for _, metric := range seen {
		results = append(results, metric)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Name != results[j].Name {
			return results[i].Name < results[j].Name
		}
		return strings.Join(results[i].Namespace, "/") < strings.Join(results[j].Namespace, "/")
	})

	return
}
