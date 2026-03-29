package sender

import (
	"context"
	"net/http"
	"sdsyslog/internal/global"
	metricGlb "sdsyslog/internal/metrics"
	"sdsyslog/internal/sender/metrics"
	"sdsyslog/internal/sender/shared"
	"sdsyslog/pkg/protocol"
	"sync"
	"time"
)

type JSONConfig struct {
	PublicKey      string `json:"publicKey"`
	SigningKeyFile string `json:"signingKeyFile,omitempty"`
	Network        struct {
		Address        string `json:"address"`
		Port           int    `json:"port"`
		MaxPayloadSize int    `json:"maxPayloadSize,omitempty"`
	} `json:"network"`
	StateFile string     `json:"stateFile"`
	Inputs    JSONInputs `json:"inputs"`
	Metrics   struct {
		Interval          string `json:"collectionInterval"`
		MaxAge            string `json:"maximumRetention,omitempty"`
		EnableQueryServer bool   `json:"enableHTTPQueryServer"`
		QueryServerPort   int    `json:"HTTPQueryServerPort"`
	} `json:"metrics"`
	AutoScaling struct {
		Enabled               bool            `json:"enabled"`
		PollInterval          string          `json:"pollInterval"`
		MinOutputs            global.MinValue `json:"minOutputs,omitempty"`
		MaxOutputs            global.MaxValue `json:"maxOutputs,omitempty"`
		MinAssemblers         global.MinValue `json:"minAssemblers,omitempty"`
		MaxAssemblers         global.MaxValue `json:"maxAssemblers,omitempty"`
		MinOutputQueueSize    global.MinValue `json:"minOutputQueueSize,omitempty"`
		MaxOutputQueueSize    global.MaxValue `json:"maxOutputQueueSize,omitempty"`
		MinAssemblerQueueSize global.MinValue `json:"minAssemblerQueueSize,omitempty"`
		MaxAssemblerQueueSize global.MaxValue `json:"maxAssemblerQueueSize,omitempty"`
	} `json:"autoscaling"`
}

type JSONInputs struct {
	Include        string                              `json:"include,omitempty"`
	DropFilters    map[string][]protocol.MessageFilter `json:"dropFilters,omitempty"`
	FilePaths      []string                            `json:"filePaths,omitempty"`
	JournalEnabled bool                                `json:"journalEnabled,omitempty"`
}

type Config struct {
	path string // JSON config path

	// Crypto
	signingPrivateKey      []byte
	transportCryptoSuiteID uint8
	signatureSuiteID       uint8

	// Destination
	DestinationIP          string
	DestinationPort        int
	OverrideMaxPayloadSize int

	// Scaling settings
	AutoscaleEnabled       bool
	AutoscaleCheckInterval time.Duration

	// Source settings
	Filters                map[string][]protocol.MessageFilter
	JournalSourceEnabled   bool
	StateFilePath          string
	FileSourcePaths        []string
	SyslogSourceListenIP   string
	SyslogSourceListenPort int

	// Worker scaling boundaries
	MinOutputs    global.MinValue
	MinAssemblers global.MinValue
	MaxOutputs    global.MaxValue
	MaxAssemblers global.MaxValue

	// Queue boundaries
	MinOutputQueueSize global.MinValue
	MaxOutputQueueSize global.MaxValue

	MinAssemblerQueueSize global.MinValue
	MaxAssemblerQueueSize global.MaxValue

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
