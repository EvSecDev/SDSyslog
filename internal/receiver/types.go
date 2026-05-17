package receiver

import (
	"context"
	"io"
	"net"
	"net/http"
	"sdsyslog/internal/global"
	metricGlb "sdsyslog/internal/metrics"
	"sdsyslog/internal/parsing"
	"sdsyslog/internal/receiver/metrics"
	"sdsyslog/internal/receiver/shard/fiprrecv"
	"sdsyslog/internal/receiver/shared"
	"sync"
	"time"
)

// User supplied options
type JSONOptions struct {
	PrivateKeyFile        string `json:"privateKeyFile"`
	PinnedSigningKeysPath string `json:"senderSigningKeysFile,omitempty"`
	Crypto                struct {
		TransportSuite string `json:"transportSuite,omitempty"`
		SignatureSuite string `json:"signatureSuite,omitempty"`
	} `json:"crypto,omitempty"`
	ReplayProtection struct {
		ProtectionWindow     parsing.Duration `json:"shortTermWindow,omitempty"`      // Short term replay protection window size (For Listener)
		PastValidityWindow   parsing.Duration `json:"longTermPastWindow,omitempty"`   // Time window where old timestamps are still accepted (relative to processing time)
		FutureValidityWindow parsing.Duration `json:"longTermFutureWindow,omitempty"` // Time window where future timestamps are still accepted (relative to processing time)
	} `json:"replayProtection,omitempty"`
	State struct {
		IPCSocketDirectory string `json:"ipcSocketDirectory,omitempty"`
	} `json:"state,omitempty"`
	Network struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
	} `json:"network"`
	Outputs struct {
		FilePath               string           `json:"filePath,omitempty"`
		JournaldURL            string           `json:"journaldURL,omitempty"`
		BeatsAddress           string           `json:"beatsAddress,omitempty"`
		DBUSNotify             bool             `json:"desktopNotifications,omitempty"`
		InternalLogs           bool             `json:"internalLogs,omitempty"`
		MaxConsecutiveFailures parsing.Duration `json:"maximumConsecutiveFailures,omitempty"` // Max failures before program shutdown
	} `json:"outputs"`
	Metrics struct {
		Interval          parsing.Duration `json:"collectionInterval"`
		MaxAge            parsing.Duration `json:"maximumRetention,omitempty"`
		EnableQueryServer bool             `json:"enableHTTPQueryServer"`
		QueryServerPort   int              `json:"HTTPQueryServerPort"`
	} `json:"metrics"`
	AutoScaling struct {
		Enabled          bool             `json:"enabled"`
		PollInterval     parsing.Duration `json:"pollInterval"`
		MinListeners     global.MinValue  `json:"minListeners,omitempty"`
		MaxListeners     global.MaxValue  `json:"maxListeners,omitempty"`
		MinProcessors    global.MinValue  `json:"minProcessors,omitempty"`
		MaxProcessors    global.MaxValue  `json:"maxProcessors,omitempty"`
		MinProcQueueSize global.MinValue  `json:"minProcQueueSize,omitempty"`
		MaxProcQueueSize global.MaxValue  `json:"maxProcQueueSize,omitempty"`
		MinDefrags       global.MinValue  `json:"minAssemblers,omitempty"`
		MaxDefrags       global.MaxValue  `json:"maxAssemblers,omitempty"`
		MinOutQueueSize  global.MinValue  `json:"minOutQueueSize,omitempty"`
		MaxOutQueueSize  global.MaxValue  `json:"maxOutQueueSize,omitempty"`
	} `json:"autoscaling"`
}

// Runtime Config
type Config struct {
	// Signature Verification
	PinnedSigningKeys map[string][]byte

	// Parsed Input
	sourceSocket *net.UDPAddr
}

type Daemon struct {
	configPath string
	cfg        Config      // Runtime
	opts       JSONOptions // User options

	// Runtime
	dryRun       bool
	startTime    time.Time
	initSuccess  bool // Tie init to start
	startSuccess bool // Tie start to run(signal handler)

	// Internal-Only Outputs
	RawWriter io.WriteCloser

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
