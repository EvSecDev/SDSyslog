package sender

import (
	"context"
	"net/http"
	metricGlb "sdsyslog/internal/metrics"
	"sdsyslog/internal/sender/metrics"
	"sdsyslog/internal/sender/shared"
	"sync"
	"time"
)

type JSONConfig struct {
	PublicKey string `json:"publicKey"`
	Network   struct {
		Address        string `json:"address"`
		Port           int    `json:"port"`
		MaxPayloadSize int    `json:"maxPayloadSize,omitempty"`
	} `json:"network"`
	StateFile string `json:"stateFile"`
	Inputs    struct {
		FilePaths      []string `json:"filePaths,omitempty"`
		JournalEnabled bool     `json:"journalEnabled,omitempty"`
	} `json:"inputs"`
	Metrics struct {
		Interval          string `json:"collectionInterval"`
		MaxAge            string `json:"maximumRetention,omitempty"`
		EnableQueryServer bool   `json:"enableHTTPQueryServer"`
		QueryServerPort   int    `json:"HTTPQueryServerPort"`
	} `json:"metrics"`
	AutoScaling struct {
		Enabled               bool   `json:"enabled"`
		PollInterval          string `json:"pollInterval"`
		MinOutputs            int    `json:"minOutputs,omitempty"`
		MaxOutputs            int    `json:"maxOutputs,omitempty"`
		MinAssemblers         int    `json:"minAssemblers,omitempty"`
		MaxAssemblers         int    `json:"maxAssemblers,omitempty"`
		MinOutputQueueSize    int    `json:"minOutputQueueSize,omitempty"`
		MaxOutputQueueSize    int    `json:"maxOutputQueueSize,omitempty"`
		MinAssemblerQueueSize int    `json:"minAssemblerQueueSize,omitempty"`
		MaxAssemblerQueueSize int    `json:"maxAssemblerQueueSize,omitempty"`
	} `json:"autoscaling"`
}

type Config struct {
	// Destination
	DestinationIP          string
	DestinationPort        int
	OverrideMaxPayloadSize int

	// Scaling settings
	AutoscaleEnabled       bool
	AutoscaleCheckInterval time.Duration

	// Source settings
	JournalSourceEnabled   bool
	StateFilePath          string
	FileSourcePaths        []string
	SyslogSourceListenIP   string
	SyslogSourceListenPort int

	// Worker scaling boundaries
	MinOutputs    int
	MinAssemblers int
	MaxOutputs    int
	MaxAssemblers int

	// Queue boundaries
	MinOutputQueueSize int
	MaxOutputQueueSize int

	MinAssemblerQueueSize int
	MaxAssemblerQueueSize int

	// Metrics
	MetricQueryServerEnabled bool
	MetricQueryServerPort    int
	MetricCollectionInterval time.Duration
	MetricMaxAge             time.Duration
}

type Daemon struct {
	cfg    Config
	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup

	// Pipeline component trackers (reverse order)
	Mgrs               shared.Managers
	metricsCollector   *metrics.Gatherer
	MetricServer       *http.Server
	MetricDataSearcher func(name string, namespacePrefix []string, start, end time.Time) []metricGlb.Metric
	MetricDiscoverer   func(name, description string, namespacePrefix []string, unit string, metricType metricGlb.MetricType) []metricGlb.Metric
	MetricAggregator   func(aggType string, name string, namespace []string, start, end time.Time) (result metricGlb.Metric, err error)
}
