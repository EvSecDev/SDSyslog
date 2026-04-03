package receiver

import (
	"context"
	"io"
	"net/http"
	"sdsyslog/internal/global"
	metricGlb "sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/metrics"
	"sdsyslog/internal/receiver/shard/fiprrecv"
	"sdsyslog/internal/receiver/shared"
	"sync"
	"time"
)

type JSONConfig struct {
	PrivateKeyFile        string `json:"privateKeyFile"`
	PinnedSigningKeysPath string `json:"senderSigningKeysFile,omitempty"`
	Network               struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
	} `json:"network"`
	Outputs struct {
		FilePath     string `json:"filePath,omitempty"`
		JournaldURL  string `json:"journaldURL,omitempty"`
		BeatsAddress string `json:"beatsAddress,omitempty"`
		DBUSNotify   bool   `json:"desktopNotifications,omitempty"`
	} `json:"outputs"`
	Metrics struct {
		Interval          string `json:"collectionInterval"`
		MaxAge            string `json:"maximumRetention,omitempty"`
		EnableQueryServer bool   `json:"enableHTTPQueryServer"`
		QueryServerPort   int    `json:"HTTPQueryServerPort"`
	} `json:"metrics"`
	AutoScaling struct {
		Enabled          bool            `json:"enabled"`
		PollInterval     string          `json:"pollInterval"`
		MinListeners     global.MinValue `json:"minListeners,omitempty"`
		MaxListeners     global.MaxValue `json:"maxListeners,omitempty"`
		MinProcessors    global.MinValue `json:"minProcessors,omitempty"`
		MaxProcessors    global.MaxValue `json:"maxProcessors,omitempty"`
		MinProcQueueSize global.MinValue `json:"minProcQueueSize,omitempty"`
		MaxProcQueueSize global.MaxValue `json:"maxProcQueueSize,omitempty"`
		MinDefrags       global.MinValue `json:"minAssemblers,omitempty"`
		MaxDefrags       global.MaxValue `json:"maxAssemblers,omitempty"`
		MinOutQueueSize  global.MinValue `json:"minOutQueueSize,omitempty"`
		MaxOutQueueSize  global.MaxValue `json:"maxOutQueueSize,omitempty"`
	} `json:"autoscaling"`
}

type Config struct {
	path string // JSON config path

	// Basic settings
	ListenIP   string
	ListenPort int

	// Crypto
	transportCryptoSuiteID uint8

	// Signature Verification
	PinnedSigningKeysFile string
	PinnedSigningKeys     map[string][]byte

	// Paths
	SocketDirectoryPath string

	// Scaling settings
	AutoscaleEnabled       bool
	AutoscaleCheckInterval time.Duration

	// Worker scaling boundaries
	MinListeners  global.MinValue
	MaxListeners  global.MaxValue
	MinProcessors global.MinValue
	MaxProcessors global.MaxValue
	MinDefrags    global.MinValue
	MaxDefrags    global.MaxValue

	// Queue boundaries
	MinOutputQueueSize    global.MinValue
	MaxOutputQueueSize    global.MaxValue
	ShardBufferSize       int
	MinProcessorQueueSize global.MinValue
	MaxProcessorQueueSize global.MaxValue

	// Message validity
	ReplayProtectionWindow time.Duration // Short term replay protection window size (For Listener)
	PastValidityWindow     time.Duration // Time window where old timestamps are still accepted (relative to processing time)
	FutureValidityWindow   time.Duration // Time window where future timestamps are still accepted (relative to processing time)

	// Outputs
	OutputFilePath string
	JournaldURL    string
	BeatsEndpoint  string
	RawWriter      io.WriteCloser
	DBUSNotify     bool

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

	Mgrs               shared.Managers
	fipr               *fiprrecv.Instance
	metricsCollector   *metrics.Gatherer
	MetricServer       *http.Server
	MetricDataSearcher func(name string, namespacePrefix []string, start, end time.Time) []metricGlb.Metric
	MetricDiscoverer   func(name, description string, namespacePrefix []string, unit string, metricType metricGlb.MetricType) []metricGlb.Metric
	MetricAggregator   func(aggType string, name string, namespace []string, start, end time.Time) (result metricGlb.Metric, err error)
}
