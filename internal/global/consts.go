package global

import "time"

const (
	// Descriptive Names for available verbosity levels
	VerbosityNone int = iota
	VerbosityStandard
	VerbosityProgress
	VerbosityData
	VerbosityFullData
	VerbosityDebug

	// Descriptive names for available severity levels
	ErrorLog string = "Error"
	WarnLog  string = "Warn"
	InfoLog  string = "Info"
)

const (
	ProgVersion string = "v0.14.3"

	// Context keys
	LoggerKey  CtxKey = "logger"  // Event queue (mostly for variable log verbosity handling)
	LogTagsKey CtxKey = "logtags" // List of tags in order of broad->specific appended/popped at various parts of the program

	ProgBaseName             string        = "sdsyslog"
	DefaultBinaryPath        string        = "/usr/local/bin/" + ProgBaseName
	DefaultConfigDir         string        = "/etc/" + ProgBaseName
	DefaultConfigSend        string        = DefaultConfigDir + "/" + ProgBaseName + "-sender.json"
	DefaultConfigRecv        string        = DefaultConfigDir + "/" + ProgBaseName + ".json"
	DefaultPrivKeyPath       string        = DefaultConfigDir + "/private.key"
	DefaultAAProfName        string        = "usr.local.bin." + ProgBaseName
	DefaultStateDir          string        = "/var/cache/" + ProgBaseName
	DefaultStateFile         string        = DefaultStateDir + "/last.state"
	DefaultReceiverPort      int           = 8514
	DefaultMinQueueSize      int           = 512
	DefaultMaxQueueSize      int           = 4096
	DefaultMinPacketDeadline time.Duration = 50 * time.Millisecond
	DefaultMaxPacketDeadline time.Duration = 1 * time.Second

	// Custom Fields (internally required, not protocol required)
	IOCtxKey    string = "SourceSink" // Identifying namespace of an in-module
	CFfacility  string = "Facility"
	CFseverity  string = "Severity"
	CFprocessid string = "ProcessID"
	CFappname   string = "ApplicationName"

	// IO Modules
	DefaultJournaldURL string = "http://localhost:19532"
	DefaultBeatsAddr   string = "localhost:5044"

	// Parsing defaults
	DefaultFacility string = "daemon"
	DefaultSeverity string = "info"

	// Timeout values
	ReceiveShutdownTimeout time.Duration = 20 * time.Second
	SendShutdownTimeout    time.Duration = 5 * time.Second

	// Metrics
	HTTPListenPortSender   int           = 10000 + DefaultReceiverPort // Default listen port
	HTTPListenPortReceiver int           = 20000 + DefaultReceiverPort // Default listen port
	HTTPListenAddr         string        = "localhost"                 // Metric queries only exposed to local machine
	HTTPReadTimeout        time.Duration = 30 * time.Second
	HTTPWriteTimeout       time.Duration = 10 * time.Second
	HTTPIdleTimeout        time.Duration = 180 * time.Second

	DiscoveryPath   string = "/discover/"
	DataPath        string = "/data/"
	AggregationPath string = "/aggregation/"

	MetricSum string = "sum"
	MetricAvg string = "average"
	MetricMin string = "min"
	MetricMax string = "max"

	// Namespacing Name Components
	NSMetric    string = "Metrics"
	NSMetricSrv string = "Server"
	NSTest      string = "Test"
	NSCLI       string = "CLI"
	NSRecv      string = "Receiver"
	NSSend      string = "Sender"
	NSProc      string = "Processor"
	NSAssm      string = "Assembler"
	NSOut       string = "Output"
	NSQueue     string = "Queue"
	NSListen    string = "Listener"
	NSWorker    string = "Worker"
	NSWatcher   string = "Watcher"
	NSmIngest   string = "Ingest"
	NSmInput    string = "In"
	NSmOutput   string = "Out"
	NSmPack     string = "Packaging"
	NSmDefrag   string = "Defrag"
	NSoFile     string = "File"
	NSoStdIn    string = "Stdin"
	NSoJrnl     string = "Journal"
	NSoRaw      string = "Raw"
)
