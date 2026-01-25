package install

import (
	"fmt"
	"os"
	"os/exec"
	"sdsyslog/internal/ebpf"
	"sdsyslog/internal/global"
	"strings"
)

// If apparmor LSM is available on this system and running as root, auto install the profile - failures are not printed under normal verbosity
func installAAProfile() (err error) {
	const appArmorProfilePath string = "/etc/apparmor.d/" + global.DefaultAAProfName
	appArmorProfile, err := installationFiles.ReadFile("static-files/" + global.DefaultAAProfName)
	if err != nil {
		err = fmt.Errorf("Unable to retrieve configuration file from embedded filesystem: %v", err)
		return
	}

	// Inject variables into config
	newaaProf := strings.Replace(string(appArmorProfile), "=$executableFilePath", "="+global.DefaultBinaryPath, 1)
	newaaProf = strings.Replace(newaaProf, "=$configurationDirPath", "="+global.DefaultConfigDir, 1)
	newaaProf = strings.Replace(newaaProf, "=$privateKeyFilePath", "="+global.DefaultPrivKeyPath, 1)
	newaaProf = strings.Replace(newaaProf, "=$progStateDirPath", "="+global.DefaultStateDir, 1)
	newaaProf = strings.Replace(newaaProf, "=$drainingSocketsMapPinPath", "="+ebpf.KernelDrainMapPath, 1)
	newaaProf = strings.Replace(newaaProf, "=$drainingSocketsFuncPinPath", "="+ebpf.KernelSocketRouteFunc, 1)
	appArmorProfile = []byte(newaaProf)

	// Check if apparmor /sys path exists
	systemAAPath := "/sys/kernel/security/apparmor/profiles"
	_, err = os.Stat(systemAAPath)
	if os.IsNotExist(err) {
		fmt.Printf("AppArmor not supported by this system\n")
		err = nil
		return
	} else if err != nil {
		err = fmt.Errorf("Unable to check if AppArmor is supported by this system: %v", err)
		return
	}

	// Write Apparmor Profile to /etc
	err = os.WriteFile(appArmorProfilePath, appArmorProfile, 0644)
	if err != nil {
		err = fmt.Errorf("Failed to write apparmor profile: %v", err)
		return
	}

	// Enact Profile
	command := exec.Command("apparmor_parser", "-r", appArmorProfilePath)
	output, err := command.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Failed to reload apparmor profile: %v: %s", err, string(output))
		return
	}

	fmt.Printf("Successfully installed AppArmor Profile\n")
	return
}

func uninstallAAProfile() (err error) {
	const appArmorProfilePath string = "/etc/apparmor.d/" + global.DefaultAAProfName

	// Check if apparmor /sys path exists
	systemAAPath := "/sys/kernel/security/apparmor/profiles"
	_, err = os.Stat(systemAAPath)
	if os.IsNotExist(err) {
		err = nil
		return
	} else if err != nil {
		err = fmt.Errorf("Unable to check if AppArmor is supported by this system: %v", err)
		return
	}

	// Warn about confined apparmor profile for uninstall
	if strings.Contains(os.Args[0], global.DefaultBinaryPath) {
		fmt.Printf("WARNING: uninstall will fail if calling this binary from within the apparmor profile\n")
		fmt.Printf("  Run this command and retry the uninstall: 'apparmor_parser -R %s'\n", appArmorProfilePath)
	}

	// Remove Profile
	command := exec.Command("apparmor_parser", "-R", appArmorProfilePath)
	output, err := command.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "not found, skipping") {
			err = fmt.Errorf("Failed to disable apparmor profile: %v: %s", err, string(output))
			return
		}
		err = nil
	}

	// Remove Apparmor Profile File
	err = os.Remove(appArmorProfilePath)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("Failed to remove apparmor profile: %v", err)
		return
	} else {
		err = nil
	}

	fmt.Printf("Successfully uninstalled AppArmor Profile\n")
	return
}
