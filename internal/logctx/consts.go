package logctx

const (
	// Descriptive Names for available verbosity levels
	VerbosityNone int = iota
	VerbosityStandard
	VerbosityProgress
	VerbosityData
	VerbosityFullData
	VerbosityDebug

	// Context keys
	LoggerKey  CtxKey = "logger"  // Event queue (mostly for variable log verbosity handling)
	LogTagsKey CtxKey = "logtags" // List of tags in order of broad->specific appended/popped at various parts of the program

	// Descriptive names for available severity levels
	FatalLog string = "Fatal"
	ErrorLog string = "Error"
	WarnLog  string = "Warn"
	InfoLog  string = "Info"

	// Namespacing Name Components
	NSMetricData      string = "Data"
	NSMetricAgg       string = "Aggregation"
	NSMetricDiscovery string = "Discovery"
	NSMetricBulk      string = "Bulk"
	NSMetric          string = "Metrics"
	NSMetricSrv       string = "Server"
	NSTest            string = "Test"
	NSCLI             string = "CLI"
	NSRecv            string = "Receiver"
	NSSend            string = "Sender"
	NSProc            string = "Processor"
	NSAssm            string = "Assembler"
	NSOut             string = "Output"
	NSQueue           string = "Queue"
	NSListen          string = "Listener"
	NSWorker          string = "Worker"
	NSWatcher         string = "Watcher"
	NSmIngest         string = "Ingest"
	NSmInput          string = "In"
	NSmOutput         string = "Out"
	NSmPack           string = "Packaging"
	NSmDefrag         string = "Defrag"
	NSmFIPR           string = "FIPR"
	NSoFile           string = "File"
	NSoStdIn          string = "Stdin"
	NSoJrnl           string = "Journal"
	NSoRaw            string = "Raw"
)
