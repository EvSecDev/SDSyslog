// Handles all installation/setup/configuration/updates
package install

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sdsyslog/internal/global"
	"strings"

	"golang.org/x/term"
)

// Read in installation static files at compile time
//
//go:embed static-files/*
var installationFiles embed.FS

// Full installation (idempotent)
func Run(mode string) {
	// Must run as root
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "Installation must be run as root\n")
		os.Exit(1)
	}

	// Move binary (self) into place
	err := installBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error installing binary: %v\n", err)
		os.Exit(1)
	}

	// Add shell autocomplete
	err = installBashAutocomplete()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting bash autocomplete: %v\n", err)
		os.Exit(1)
	}

	// Create template config
	err = installConfig(mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with template config: %v\n", err)
		os.Exit(1)
	}

	// Create apparmor profile if system supports it
	err = installAAProfile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with AppArmor profile: %v\n", err)
		os.Exit(1)
	}

	// Create systemd service
	err = installService(mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with Systemd service: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installation completed successfully\n")
}

// Full uninstall
func Remove(mode string) {
	// Only ask if in terminal
	if term.IsTerminal(int(os.Stdout.Fd())) {
		// File exists, prompt user for confirmation to overwrite
		fmt.Printf("Are you SURE you want to uninstall? (this will remove the configuration files) (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
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

	// Cleanup state dir - best effort
	stateDir := filepath.Dir(global.DefaultStateFile)
	os.Remove(global.DefaultStateFile)
	os.Remove(stateDir)
}
