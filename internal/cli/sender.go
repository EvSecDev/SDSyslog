package cli

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/sender"
)

func SendMode(ctx context.Context, cliOpts *global.CommandSet, commandname string, args []string) {
	var configPath string
	commandFlags := flag.NewFlagSet(commandname, flag.ExitOnError)
	requestedLogLevel := SetGlobalArguments(commandFlags)
	SetCommon(commandFlags, &configPath, commandname)

	commandFlags.Usage = func() {
		PrintHelpMenu(commandFlags, commandname, cliOpts)
	}
	if len(args) < 1 {
		PrintHelpMenu(commandFlags, commandname, cliOpts)
		os.Exit(1)
	}
	commandFlags.Parse(args[0:])

	// Change log level if verbosity argument was given at this command level
	logctx.SetLogLevel(ctx, *requestedLogLevel)

	// Embed mode name in context
	ctx = context.WithValue(ctx, global.CtxModeKey, commandname)

	jsonCfg, err := sender.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	publicKey, err := base64.StdEncoding.DecodeString(jsonCfg.PublicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding public key: %v\n", err)
		os.Exit(1)
	}

	daemonConfig, err := jsonCfg.NewDaemonConf()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sendDaemon := sender.NewDaemon(daemonConfig)
	err = sendDaemon.Start(ctx, publicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting sending daemon: %v\n", err)
		os.Exit(1)
	}

	sendDaemon.Run()
}
