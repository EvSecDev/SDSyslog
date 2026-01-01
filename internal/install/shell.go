package install

import (
	"fmt"
	"os"
	"path/filepath"
)

func installBashAutocomplete() (err error) {
	const sysAutocompleteDir string = "/usr/share/bash-completion/completions"
	autoCompleteFunc, err := installationFiles.ReadFile("static-files/autocomplete.sh")
	if err != nil {
		err = fmt.Errorf("Unable to retrieve autocomplete file from embedded filesystem: %v", err)
		return
	}

	executablePath, err := filepath.Abs(os.Args[0])
	if err != nil {
		err = fmt.Errorf("Failed to retrieve absolute executable path for profile installation: %v", err)
		return
	}
	executableName := filepath.Base(executablePath)

	// Write to system, or fallback to users home
	var autoCompleteFilePath string
	_, err = os.Stat(sysAutocompleteDir)
	if err == nil {
		autoCompleteFilePath = filepath.Join(sysAutocompleteDir, executableName)
	} else {
		var homeDir string
		homeDir, err = os.UserHomeDir()
		if err != nil {
			err = fmt.Errorf("Failed to find user home directory: %v", err)
			return
		}
		userDir := filepath.Join(homeDir, ".bash_completion.d")
		err = os.MkdirAll(userDir, 0750)
		if err != nil {
			err = fmt.Errorf("Failed to create user autocomplete dir: %v", err)
			return
		}
		err = nil

		autoCompleteFilePath = filepath.Join(userDir, executableName)
		fmt.Printf("System completion dir missing, installing bash completion at %s\n", autoCompleteFilePath)
		fmt.Printf("Make sure ~/.bashrc sources ~/.bash_completion and ~/.bash_completion.d/*\n")
	}

	err = os.WriteFile(autoCompleteFilePath, autoCompleteFunc, 0644)
	if err != nil {
		err = fmt.Errorf("Failed to write autocompletion file: %v", err)
		return
	}
	return
}

func uninstallBashAutocomplete() (err error) {
	const sysAutocompleteDir string = "/usr/share/bash-completion/completions"

	executablePath, err := filepath.Abs(os.Args[0])
	if err != nil {
		err = fmt.Errorf("Failed to retrieve absolute executable path for profile installation: %v", err)
		return
	}
	executableName := filepath.Base(executablePath)

	// Write to system, or fallback to users home
	var autoCompleteFilePath string
	_, err = os.Stat(sysAutocompleteDir)
	if err == nil {
		autoCompleteFilePath = filepath.Join(sysAutocompleteDir, executableName)
	} else {
		var homeDir string
		homeDir, err = os.UserHomeDir()
		if err != nil {
			err = fmt.Errorf("Failed to find user home directory: %v", err)
			return
		}
		userDir := filepath.Join(homeDir, ".bash_completion.d")
		autoCompleteFilePath = filepath.Join(userDir, executableName)
	}

	err = os.Remove(autoCompleteFilePath)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("Failed to remove autocompletion file: %v", err)
		return
	} else {
		err = nil
	}

	fmt.Printf("Successfully removed shell autocompletion\n")
	return
}
