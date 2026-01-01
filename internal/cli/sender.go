package cli

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/sender"
)

func SendMode(ctx context.Context, commandname string, args []string) {
	var configPath string
	commandFlags := flag.NewFlagSet(commandname, flag.ExitOnError)
	SetGlobalArguments(commandFlags)
	SetCommon(commandFlags, &configPath)

	commandFlags.Usage = func() {
		PrintHelpMenu(commandFlags, commandname, global.CmdOpts)
	}
	if len(args) < 1 {
		PrintHelpMenu(commandFlags, commandname, global.CmdOpts)
		os.Exit(1)
	}
	commandFlags.Parse(args[0:])

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
