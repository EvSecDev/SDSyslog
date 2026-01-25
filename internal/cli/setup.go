package cli

import (
	"flag"
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/install"
)

// Setup/installation options
func SetupMode(cliOpts *global.CommandSet, commandname string, args []string) {
	var newKeyPair bool
	var newSendConf bool
	var newRecvConf bool
	var installSender bool
	var installReceiver bool
	var uninstallSender bool
	var uninstallReceiver bool
	var templateConfPath string

	commandFlags := flag.NewFlagSet(commandname, flag.ExitOnError)
	commandFlags.BoolVar(&uninstallSender, "uninstall-sender", false, "Remove the sender daemon")
	commandFlags.BoolVar(&uninstallReceiver, "uninstall-receiver", false, "Remove the receiver daemon")
	commandFlags.BoolVar(&installSender, "install-sender", false, "Install/Upgrade the sender daemon")
	commandFlags.BoolVar(&installReceiver, "install-receiver", false, "Install/Upgrade the receiver daemon")
	commandFlags.StringVar(&templateConfPath, "c", "", "Path to template config file")
	commandFlags.StringVar(&templateConfPath, "config", "", "Path to template config file")
	commandFlags.BoolVar(&newKeyPair, "create-keys", false, "Create new persistent key pair (prints to stdout)")
	commandFlags.BoolVar(&newSendConf, "send-config-template", false, "Create new template config for the sender daemon (using config-path argument)")
	commandFlags.BoolVar(&newRecvConf, "recv-config-template", false, "Create new template config for the receiver daemon (using config-path argument)")

	commandFlags.Usage = func() {
		PrintHelpMenu(commandFlags, commandname, cliOpts)
	}
	if len(args) < 1 {
		PrintHelpMenu(commandFlags, commandname, cliOpts)
		os.Exit(1)
	}
	commandFlags.Parse(args[0:])

	var err error

	if newKeyPair {
		err = install.GeneratePrivateKeys()
	} else if newSendConf {
		err = install.CreateSendTemplateConfig(templateConfPath)
	} else if newRecvConf {
		err = install.CreateRecvTemplateConfig(templateConfPath)
	} else if installSender {
		install.Run("send")
	} else if installReceiver {
		install.Run("receive")
	} else if uninstallSender {
		install.Remove("send")
	} else if uninstallReceiver {
		install.Remove("receive")
	} else {
		PrintHelpMenu(commandFlags, commandname, cliOpts)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
