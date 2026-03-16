package install

import (
	"fmt"
	"os"
	"sdsyslog/internal/global"
)

func installBinary() (err error) {
	selfPath, err := os.Executable()
	if err != nil {
		return
	}

	if selfPath == global.DefaultBinaryPath {
		// No-op
		return
	}

	err = os.Rename(selfPath, global.DefaultBinaryPath)
	if err != nil {
		err = fmt.Errorf("failed to move: %w", err)
		return
	}

	err = os.Chmod(global.DefaultBinaryPath, 0755)
	if err != nil {
		err = fmt.Errorf("failed to change permissions: %w", err)
		return
	}

	fmt.Printf("Successfully installed binary to '%s'\n", global.DefaultBinaryPath)
	return
}

func uninstallBinary() (err error) {
	err = os.Remove(global.DefaultBinaryPath)
	if err != nil && !os.IsNotExist(err) {
		return
	} else {
		err = nil
	}

	fmt.Printf("Successfully removed binary from '%s'\n", global.DefaultBinaryPath)
	return
}
