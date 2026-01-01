package cli

import "sdsyslog/internal/global"

func DefineOptions() (cmdOpts *global.CommandSet) {
	// Root level
	root := &global.CommandSet{
		Description:     "Secure Diode System Logger (SDSyslog)",
		FullDescription: "  Encrypts and transfers messages over unidirectional networks",
		CommandName:     RootCLICommand,
		ChildCommands:   make(map[string]*global.CommandSet),
	}

	// Sending
	root.ChildCommands["send"] = &global.CommandSet{
		CommandName:     "send",
		Description:     "Send Messages",
		FullDescription: "Reads messages from external sources, encrypts, fragments and transmits to configured destination",
		ChildCommands:   nil,
	}

	// Receiving
	root.ChildCommands["receive"] = &global.CommandSet{
		CommandName:     "receive",
		Description:     "Receive Messages",
		FullDescription: "Receives network packets, decrypts, reassembles, and sends messages to configured outputs",
		ChildCommands:   nil,
	}

	// Setup
	root.ChildCommands["configure"] = &global.CommandSet{
		CommandName:     "configure",
		Description:     "Setup Actions",
		FullDescription: "Configure various aspects of installation, generation, and runtime",
		ChildCommands:   nil,
	}

	// Version Info
	root.ChildCommands["version"] = &global.CommandSet{
		CommandName:     "version",
		Description:     "Show Version Information",
		FullDescription: "Display meta information about program",
	}

	cmdOpts = root
	return
}
