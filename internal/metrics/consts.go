package metrics

const (
	MetricSum string = "sum"
	MetricAvg string = "average"
	MetricMin string = "min"
	MetricMax string = "max"

	MTDropped   string = "dropped_count"
	DescDropped string = "Count of internally dropped packets/payloads/messages (that are otherwise valid)"
)
