package metrics

import (
	"fmt"
	"sdsyslog/internal/global"
	"sort"
	"strconv"
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

// Finds and aggregates all data for a given metric for the aggregation type (global consts prefixed by Metric*).
// Requires exact match for name and namespace.
// Start/end time if not provided will default to past minute.
func (registry *Registry) Aggregate(aggType string, name string, namespace []string, start, end time.Time) (result Metric, err error) {
	if start.IsZero() && end.IsZero() {
		start = time.Now().Add(-1 * time.Minute)
		end = time.Now()
	}

	aggType = strings.ToLower(aggType)

	// Grab all individual time slices for the metric
	metricsResults := registry.Search(name, namespace, start, end)
	if len(metricsResults) == 0 {
		err = fmt.Errorf("search returned no results")
		return
	}

	// Iterate over the metrics and aggregate values based on the aggregation type
	var aggregatedValue float64
	var count int
	for idx, metric := range metricsResults {
		count++

		value, ok := toFloat64(metric.Value.Raw)
		if !ok {
			err = fmt.Errorf("non-numeric value for aggregation %s: %T", aggType, metric.Value.Raw)
			return
		}

		switch aggType {
		case global.MetricSum:
			aggregatedValue += value
		case global.MetricAvg:
			aggregatedValue += value
		case global.MetricMin:
			if idx == 0 {
				aggregatedValue = value
			}

			if value < aggregatedValue {
				aggregatedValue = value
			}
		case global.MetricMax:
			if idx == 0 {
				aggregatedValue = value
			}

			if value > aggregatedValue {
				aggregatedValue = value
			}
		default:
			err = fmt.Errorf("unsupported aggregation type: %s", aggType)
			return
		}
	}

	if aggType == global.MetricAvg {
		aggregatedValue /= float64(count)
	}

	result = Metric{
		Name:        metricsResults[0].Name,
		Description: metricsResults[0].Description,
		Namespace:   metricsResults[0].Namespace,
		Type:        metricsResults[0].Type,
		Timestamp:   time.Now(),
		Value: MetricValue{
			Raw:      aggregatedValue,
			Unit:     metricsResults[0].Value.Unit,
			Interval: metricsResults[0].Value.Interval,
		},
	}
	return
}

func toFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case int32:
		return float64(t), true
	case uint:
		return float64(t), true
	case uint64:
		return float64(t), true
	case uint32:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
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
