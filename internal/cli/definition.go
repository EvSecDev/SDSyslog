package cli

import "sdsyslog/internal/global"

func DefineOptions() (cmdOpts *CommandSet) {
	// Root level
	root := &CommandSet{
		Description:     "Secure Diode System Logger (SDSyslog)",
		FullDescription: "  Encrypts and transfers messages over unidirectional networks",
		CommandName:     RootCLICommand,
		ChildCommands:   make(map[string]*CommandSet),
	}

	// Sending
	root.ChildCommands[global.SendMode] = &CommandSet{
		CommandName:     global.SendMode,
		Description:     "Send Messages",
		FullDescription: "Reads messages from external sources, encrypts, fragments and transmits to configured destination",
		ChildCommands:   nil,
	}

	// Receiving
	root.ChildCommands[global.RecvMode] = &CommandSet{
		CommandName:     global.RecvMode,
		Description:     "Receive Messages",
		FullDescription: "Receives network packets, decrypts, reassembles, and sends messages to configured outputs",
		ChildCommands:   nil,
	}

	// Setup
	root.ChildCommands["configure"] = &CommandSet{
		CommandName:     "configure",
		Description:     "Setup Actions",
		FullDescription: "Configure various aspects of installation, generation, and runtime",
		ChildCommands:   nil,
	}

	// Version Info
	root.ChildCommands["version"] = &CommandSet{
		CommandName:     "version",
		Description:     "Show Version Information",
		FullDescription: "Display meta information about program",
	}

	cmdOpts = root
	return
}
