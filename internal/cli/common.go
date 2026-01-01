package cli

import (
	"flag"
	"sdsyslog/internal/global"
)

func SetGlobalArguments(fs *flag.FlagSet) {
	fs.IntVar(&global.Verbosity, "v", 1, "Increase detailed progress messages (Higher is more verbose) <0...5>")
	fs.IntVar(&global.Verbosity, "verbosity", 1, "Increase detailed progress messages (Higher is more verbose) <0...5>")
}

func SetCommon(fs *flag.FlagSet, configPath *string) {
	fs.StringVar(configPath, "c", global.DefaultConfigPath, "Path to the configuration file")
	fs.StringVar(configPath, "config", global.DefaultConfigPath, "Path to the configuration file")
}
