package install

import "sdsyslog/internal/global"

const (
	// Crypto
	encryptionPrivKeyPath string = global.DefaultConfigDir + "/private.key"

	// Systemd
	systemdUnitDir   string = "/etc/systemd/system/"
	senderUnitPath   string = systemdUnitDir + global.ProgBaseName + "-sender.service"
	receiverUnitPath string = systemdUnitDir + global.ProgBaseName + ".service"

	// Shell
	sysAutocompleteDir string = "/usr/share/bash-completion/completions"

	// Apparmor
	sysAAProfilePath    string = "/sys/kernel/security/apparmor/profiles"
	apparmorProfDir     string = "/etc/apparmor.d/"
	apparmorProfName    string = "usr.local.bin." + global.ProgBaseName
	appArmorProfilePath string = apparmorProfDir + apparmorProfName
)
