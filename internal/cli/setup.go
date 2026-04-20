package cli

import (
	"flag"
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/setup"
)

// Setup/installation options
func SetupMode(cliOpts *CommandSet, commandname string, args []string) {
	var newKeyPair bool
	var newSigningKeys bool
	var newSendConf bool
	var newRecvConf bool
	var installSender bool
	var installReceiver bool
	var uninstallSender bool
	var uninstallReceiver bool
	var confPath string
	var dryRun bool
	var verbose bool

	commandFlags := flag.NewFlagSet(commandname, flag.ExitOnError)
	commandFlags.BoolVar(&uninstallSender, "uninstall-sender", false, "Remove the sender daemon")
	commandFlags.BoolVar(&uninstallReceiver, "uninstall-receiver", false, "Remove the receiver daemon")
	commandFlags.BoolVar(&installSender, "install-sender", false, "Install/Upgrade the sender daemon")
	commandFlags.BoolVar(&installReceiver, "install-receiver", false, "Install/Upgrade the receiver daemon")
	commandFlags.StringVar(&confPath, "c", "", "Path to config file")
	commandFlags.StringVar(&confPath, "config", "", "Path to config file")
	commandFlags.BoolVar(&newKeyPair, "create-keys", false, "Create new persistent key pair (prints to stdout)")
	commandFlags.BoolVar(&newSigningKeys, "create-signing-keys", false, "Create new persistent signing key pair (prints to stdout)")
	commandFlags.BoolVar(&newSendConf, "send-config-template", false, "Create new template config for the sender daemon (using config-path argument)")
	commandFlags.BoolVar(&newRecvConf, "recv-config-template", false, "Create new template config for the receiver daemon (using config-path argument)")
	commandFlags.BoolVar(&dryRun, "T", false, "No not mutate anything, but print what would have been done")
	commandFlags.BoolVar(&dryRun, "dry-run", false, "No not mutate anything, but print what would have been done")
	commandFlags.BoolVar(&verbose, "v", false, "Print detailed progress messages")
	commandFlags.BoolVar(&verbose, "verbose", false, "Print detailed progress messages")

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

	var defaultSuiteID uint8 = 1

	if newKeyPair {
		err = setup.GeneratePrivateKeys(defaultSuiteID)
	} else if newSigningKeys {
		err = setup.GenerateSigningKeys(defaultSuiteID)
	} else if newSendConf {
		err = setup.CreateSendTemplateConfig(confPath)
	} else if newRecvConf {
		err = setup.CreateRecvTemplateConfig(confPath)
	} else if installSender {
		var inst *setup.Installer
		inst, err = setup.NewInstaller(global.SendMode, defaultSuiteID, dryRun, verbose)
		if err == nil {
			err = inst.RunInstall()
		}
	} else if installReceiver {
		var inst *setup.Installer
		inst, err = setup.NewInstaller(global.RecvMode, defaultSuiteID, dryRun, verbose)
		if err == nil {
			err = inst.RunInstall()
		}
	} else if uninstallSender {
		var inst *setup.Installer
		inst, err = setup.NewInstaller(global.SendMode, defaultSuiteID, dryRun, verbose)
		if err == nil {
			err = inst.RunUninstall()
		}
	} else if uninstallReceiver {
		var inst *setup.Installer
		inst, err = setup.NewInstaller(global.RecvMode, defaultSuiteID, dryRun, verbose)
		if err == nil {
			err = inst.RunUninstall()
		}
	} else {
		PrintHelpMenu(commandFlags, commandname, cliOpts)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
