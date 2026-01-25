package cli

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver"
)

func ReceiveMode(ctx context.Context, cliOpts *global.CommandSet, commandname string, args []string) {
	var configPath string
	commandFlags := flag.NewFlagSet(commandname, flag.ExitOnError)
	requestedLogLevel := SetGlobalArguments(commandFlags)
	SetCommon(commandFlags, &configPath, "receive")

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

	jsonCfg, err := receiver.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	privateKey, err := os.ReadFile(jsonCfg.PrivateKeyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading private key file: %v\n", err)
		os.Exit(1)
	}

	key, err := base64.StdEncoding.DecodeString(string(privateKey))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding private key: %v\n", err)
		os.Exit(1)
	}

	daemonConfig, err := jsonCfg.NewDaemonConf()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	recvDaemon := receiver.NewDaemon(daemonConfig)
	err = recvDaemon.Start(ctx, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting receiving daemon: %v\n", err)
		os.Exit(1)
	}

	recvDaemon.Run()
}
