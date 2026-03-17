// Handles all installation/setup/configuration/updates
package install

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"strings"

	"golang.org/x/term"
)

// Read in installation static files at compile time
//
//go:embed static-files/*
var installationFiles embed.FS

// Full installation (idempotent)
func Run(mode string) (err error) {
	// Must run as root
	if os.Geteuid() != 0 {
		err = fmt.Errorf("installation must be run as root")
		return
	}

	// Move binary (self) into place
	err = installBinary()
	if err != nil {
		err = fmt.Errorf("executable file: %w", err)
		return
	}

	// Add shell autocomplete
	err = installBashAutocomplete()
	if err != nil {
		err = fmt.Errorf("bash autocomplete: %w", err)
		return
	}

	// Create template config
	err = installConfig(mode)
	if err != nil {
		err = fmt.Errorf("configuration file: %w", err)
		return
	}

	// Create apparmor profile if system supports it
	err = installAAProfile()
	if err != nil {
		err = fmt.Errorf("apparmor: %w", err)
		return
	}

	// Create systemd service
	err = installService(mode)
	if err != nil {
		err = fmt.Errorf("systemd: %w", err)
		return
	}

	fmt.Printf("Installation completed successfully\n")
	return
}

// Full uninstall
func Remove(mode string) {
	// Only ask if in terminal
	if term.IsTerminal(int(os.Stdin.Fd())) {
		// File exists, prompt user for confirmation to overwrite
		fmt.Printf("Are you SURE you want to uninstall? (this will remove the configuration files) (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		input = strings.TrimSpace(input)

		if strings.ToLower(input) != "yes" {
			fmt.Printf("Aborting uninstall\n")
			return
		}
	}

	// Must run as root
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "Uninstall must be run as root\n")
		os.Exit(1)
	}

	// Remove apparmor profile if system supports it
	err := uninstallAAProfile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with AppArmor profile: %v\n", err)
	}

	// Systemd service
	err = uninstallService(mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with Systemd service: %v\n", err)
	}

	// Remove binary
	err = uninstallBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error removing binary: %v\n", err)
	}

	// Remove shell autocomplete
	err = uninstallBashAutocomplete()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error removing bash autocomplete: %v\n", err)
	}

	// Remove template config
	err = uninstallConfig(mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with template config: %v\n", err)
	}

	// Cleanup state dir
	err = os.RemoveAll(global.DefaultStateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with removing state dir: %v\n", err)
	}
}
