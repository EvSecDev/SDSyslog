package receiver

import (
	"context"
	"net/http"
	metricGlb "sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/metrics"
	"sdsyslog/internal/receiver/shared"
	"sync"
	"time"
)

type JSONConfig struct {
	PrivateKeyFile string `json:"privateKeyFile"`
	Network        struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
	} `json:"network"`
	Outputs struct {
		FilePath       string `json:"filePath,omitempty"`
		JournalEnabled bool   `json:"journalEnabled,omitempty"`
	} `json:"outputs"`
	Metrics struct {
		Interval          string `json:"collectionInterval"`
		MaxAge            string `json:"maximumRetention,omitempty"`
		EnableQueryServer bool   `json:"enableHTTPQueryServer"`
	} `json:"metrics"`
	AutoScaling struct {
		Enabled          bool   `json:"enabled"`
		PollInterval     string `json:"pollInterval"`
		MinListeners     int    `json:"minListeners,omitempty"`
		MaxListeners     int    `json:"maxListeners,omitempty"`
		MinProcessors    int    `json:"minProcessors,omitempty"`
		MaxProcessors    int    `json:"maxProcessors,omitempty"`
		MinProcQueueSize int    `json:"minProcQueueSize,omitempty"`
		MaxProcQueueSize int    `json:"maxProcQueueSize,omitempty"`
		MinDefrags       int    `json:"minAssemblers,omitempty"`
		MaxDefrags       int    `json:"maxAssemblers,omitempty"`
	} `json:"autoscaling"`
}

type Config struct {
	// Basic settings
	ListenIP   string
	ListenPort int

	// Scaling settings
	AutoscaleEnabled       bool
	AutoscaleCheckInterval time.Duration

	// Worker scaling boundaries
	MinListeners  int
	MaxListeners  int
	MinProcessors int
	MaxProcessors int
	MinDefrags    int
	MaxDefrags    int

	// Queue boundaries
	MinOutputQueueSize    int
	MaxOutputQueueSize    int
	ShardBufferSize       int
	MinProcessorQueueSize int
	MaxProcessorQueueSize int

	// Outputs
	OutputFilePath string
	JournalEnabled bool

	// Metrics
	MetricQueryServerEnabled bool
	MetricCollectionInterval time.Duration
	MetricMaxAge             time.Duration
}

type Daemon struct {
	cfg    Config
	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup

	Mgrs               shared.Managers
	metricsCollector   *metrics.Gatherer
	MetricServer       *http.Server
	MetricDataSearcher func(name string, namespacePrefix []string, start, end time.Time) []metricGlb.Metric
	MetricDiscoverer   func(name, description string, namespacePrefix []string, unit string, metricType metricGlb.MetricType) []metricGlb.Metric
}
