package cli

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver"
)

func ReceiveMode(ctx context.Context, cliOpts *CommandSet, commandname string, args []string) {
	var configPath string
	var addPinnedKey string
	var delPinnedKey string

	commandFlags := flag.NewFlagSet(commandname, flag.ExitOnError)
	requestedLogLevel := SetGlobalArguments(commandFlags)
	SetCommon(commandFlags, &configPath, commandname)
	commandFlags.StringVar(&addPinnedKey, "trust-sender", "", "Add a pinned public key for a sender (format: <hostname>"+receiver.PinedKeysReqSeparator+"<base64 key|pem file>)")
	commandFlags.StringVar(&delPinnedKey, "distrust-sender", "", "Remove a pinned public key for the given sender hostname")

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

	// Protect listener syscall actions failing for platforms not supported
	if runtime.GOOS != global.GOOSLinux {
		fmt.Fprintf(os.Stderr, "Error: receive mode is not supported on OS %q\n", runtime.GOOS)
		os.Exit(1)
	}

	// Configuration options (non-daemon)
	if addPinnedKey != "" {
		err = receiver.AddPinnedKey(configPath, addPinnedKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if delPinnedKey != "" {
		err = receiver.RemovePinnedKey(configPath, delPinnedKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

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

	daemonConfig, err := jsonCfg.NewDaemonConf(configPath)
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
