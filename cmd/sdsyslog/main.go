package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sdsyslog/internal/cli"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
)

func main() {
	cliOpts := cli.DefineOptions()

	args := os.Args
	commandFlags := flag.NewFlagSet(args[0], flag.ExitOnError)
	requestedLogLevel := cli.SetGlobalArguments(commandFlags)

	commandFlags.Usage = func() {
		cli.PrintHelpMenu(commandFlags, cli.RootCLICommand, cliOpts)
	}
	if len(args) < 2 {
		cli.PrintHelpMenu(commandFlags, cli.RootCLICommand, cliOpts)
		os.Exit(1)
	}
	commandFlags.Parse(args[1:])

	// Retrieve command and args
	command := args[1]
	args = args[2:]

	// Setting global logging
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := logctx.NewLogger("global", *requestedLogLevel, ctx.Done()) // New logger tied to global
	ctx = logctx.WithLogger(ctx, logger)                                 // Add logger to global ctx
	logctx.StartWatcher(logger, os.Stdout)                               // Send received output to stdout

	// Process commands
	switch command {
	case "send":
		cli.SendMode(ctx, cliOpts, command, args)
	case "receive":
		cli.ReceiveMode(ctx, cliOpts, command, args)
	case "configure":
		cli.SetupMode(cliOpts, command, args)
	case "version":
		if len(args) > 0 && (args[0] == "--verbosity" || args[0] == "-v") {
			fmt.Printf("SDSyslog %s\n", global.ProgVersion)
			fmt.Printf("Built using %s(%s) for %s on %s\n", runtime.Version(), runtime.Compiler, runtime.GOOS, runtime.GOARCH)
			fmt.Print("License GPLv3+: GNU GPL version 3 or later <https://gnu.org/licenses/gpl.html>\n")
		} else {
			fmt.Println(global.ProgVersion)
		}
	default:
		cli.PrintHelpMenu(commandFlags, "root", cliOpts)
		os.Exit(1)
	}

	// Finish up any stdout writes for global logger
	cancel()
	logger.Wake()
	logger.Wait()
}
