package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/sender"
)

func SendMode(ctx context.Context, cliOpts *CommandSet, commandname string, args []string) {
	var configPath string
	var testConfig bool
	var writeSigningKey bool
	commandFlags := flag.NewFlagSet(commandname, flag.ExitOnError)
	requestedLogLevel := SetGlobalArguments(commandFlags)
	SetCommon(commandFlags, &configPath, commandname)
	commandFlags.BoolVar(&testConfig, "t", false, "Test configuration and exit")
	commandFlags.BoolVar(&testConfig, "test-config", false, "Test configuration and exit")
	commandFlags.BoolVar(&writeSigningKey, "write-signing-key", false, "Write new private signing key supplied via stdin to config")

	commandFlags.Usage = func() {
		PrintHelpMenu(commandFlags, commandname, cliOpts)
	}
	if len(args) < 1 {
		PrintHelpMenu(commandFlags, commandname, cliOpts)
		os.Exit(1)
	}
	err := commandFlags.Parse(args[0:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Change log level if verbosity argument was given at this command level
	logctx.SetLogLevel(ctx, *requestedLogLevel)

	// Embed mode name in context
	ctx = context.WithValue(ctx, global.CtxModeKey, commandname)

	if writeSigningKey {
		err := sender.WriteNewSigningKey(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	sendDaemon := sender.NewDaemon(ctx, testConfig)
	err = sendDaemon.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	publicKey, err := sendDaemon.LoadPubKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	err = sendDaemon.Init(publicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing sending daemon: %v\n", err)
		os.Exit(1)
	}
	err = sendDaemon.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting sending daemon: %v\n", err)
		os.Exit(1)
	}

	if testConfig {
		return
	}

	sendDaemon.Run()
}
