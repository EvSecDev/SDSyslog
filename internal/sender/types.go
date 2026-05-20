package sender

import (
	"context"
	"io"
	"net"
	"net/http"
	"sdsyslog/internal/global"
	metricGlb "sdsyslog/internal/metrics"
	"sdsyslog/internal/parsing"
	"sdsyslog/internal/sender/metrics"
	"sdsyslog/internal/sender/shared"
	"sdsyslog/pkg/protocol"
	"sync"
	"time"
)

// User supplied options
type JSONOptions struct {
	PublicKey      string `json:"publicKey"`
	SigningKeyFile string `json:"signingKeyFile,omitempty"`
	Crypto         struct {
		TransportSuite string `json:"transportSuite,omitempty"`
		SignatureSuite string `json:"signatureSuite,omitempty"`
	} `json:"crypto,omitempty"`
	State struct {
		BaseFile string `json:"baseStateFile,omitempty"`
	} `json:"state,omitempty"`
	Network struct {
		SourceAddress          string `json:"sourceAddress,omitempty"`
		SourcePort             int    `json:"sourcePort,omitempty"`
		Address                string `json:"address"`
		Port                   int    `json:"port"`
		OverrideMaxPayloadSize int    `json:"maxPayloadSize,omitempty"`
	} `json:"network"`
	Inputs  JSONInputs `json:"inputs"`
	Metrics struct {
		Interval          parsing.Duration `json:"collectionInterval"`
		MaxAge            parsing.Duration `json:"maximumRetention,omitempty"`
		EnableQueryServer bool             `json:"enableHTTPQueryServer"`
		QueryServerPort   int              `json:"HTTPQueryServerPort"`
	} `json:"metrics"`
	AutoScaling struct {
		Enabled               bool             `json:"enabled"`
		PollInterval          parsing.Duration `json:"pollInterval"`
		MinOutputs            global.MinValue  `json:"minOutputs,omitempty"`
		MaxOutputs            global.MaxValue  `json:"maxOutputs,omitempty"`
		MinAssemblers         global.MinValue  `json:"minAssemblers,omitempty"`
		MaxAssemblers         global.MaxValue  `json:"maxAssemblers,omitempty"`
		MinOutputQueueSize    global.MinValue  `json:"minOutputQueueSize,omitempty"`
		MaxOutputQueueSize    global.MaxValue  `json:"maxOutputQueueSize,omitempty"`
		MinAssemblerQueueSize global.MinValue  `json:"minAssemblerQueueSize,omitempty"`
		MaxAssemblerQueueSize global.MaxValue  `json:"maxAssemblerQueueSize,omitempty"`
	} `json:"autoscaling"`
	Throttling struct {
		Enabled              bool             `json:"enabled"`
		MinFragmentThreshold int              `json:"minimumFragmentThreshold"`
		PerFragmentDelay     parsing.Duration `json:"perFragmentDelay"`
	} `json:"throttling"`
}

type JSONInputs struct {
	Include          string                              `json:"include,omitempty"`
	DropFilters      map[string][]protocol.MessageFilter `json:"dropFilters,omitempty"`
	FilePaths        []string                            `json:"filePaths,omitempty"`
	JournalEnabled   bool                                `json:"journalEnabled,omitempty"`
	SendInternalLogs bool                                `json:"sendInternalLogs,omitempty"`
}

type Config struct {
	// Crypto
	signingPrivateKey []byte

	// Parsed network
	sourceSocket *net.UDPAddr
	destSocket   *net.UDPAddr
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
	RawInput io.ReadCloser

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
