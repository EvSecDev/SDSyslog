package global

type CommandSet struct {
	CommandName     string                 // Exact name of cli command
	UsageOption     string                 // Expected command value in usage top line
	Description     string                 // Short text displayed on parent command
	FullDescription string                 // Long text displayed on current command
	ChildCommands   map[string]*CommandSet // Available subcommands
}

type CtxKey string
