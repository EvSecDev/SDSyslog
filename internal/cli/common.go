package cli

import (
	"flag"
	"sdsyslog/internal/global"
)

func SetGlobalArguments(fs *flag.FlagSet) (requestedLogLevel *int) {
	requestedLogLevel = new(int)
	fs.IntVar(requestedLogLevel, "v", 1, "Increase detailed progress messages (Higher is more verbose) <0...5>")
	fs.IntVar(requestedLogLevel, "verbosity", 1, "Increase detailed progress messages (Higher is more verbose) <0...5>")
	return
}

func SetCommon(fs *flag.FlagSet, configPath *string, mode string) {
	switch mode {
	case global.SendMode:
		fs.StringVar(configPath, "c", global.DefaultConfigSend, "Path to the configuration file")
		fs.StringVar(configPath, "config", global.DefaultConfigSend, "Path to the configuration file")
	case global.RecvMode:
		fs.StringVar(configPath, "c", global.DefaultConfigRecv, "Path to the configuration file")
		fs.StringVar(configPath, "config", global.DefaultConfigRecv, "Path to the configuration file")
	}
}
