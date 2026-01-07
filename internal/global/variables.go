package global

var (
	CmdOpts         *CommandSet // Holds CLI command definition
	LogicalCPUCount int         // For max workers
	Hostname        string      // local machine name
	BootID          string      // UUID of system for current runtime 
	PID             int         // self

	// Integer for printing increasingly detailed information as program progresses
	//
	//	0 - None: quiet (prints nothing but errors)
	//	1 - Standard: normal progress messages
	//	2 - Progress: more progress messages (no actual data outputted)
	//	3 - Data: shows limited data being processed
	//	4 - FullData: shows full data being processed
	//	5 - Debug: shows extra data during processing (raw bytes)
	Verbosity int
)
