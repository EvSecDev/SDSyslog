// Parent package containing all modules for sending or receiving data to/from external sources.
package externalio

const (
	// Parsing defaults
	DefaultFacility string = "daemon"
	DefaultSeverity string = "info"

	// Custom Fields (internally required, not protocol required)
	CtxKey      string = "SourceSink" // Identifying namespace of an in-module
	CFfacility  string = "Facility"
	CFseverity  string = "Severity"
	CFprocessid string = "ProcessID"
	CFappname   string = "ApplicationName"
)
