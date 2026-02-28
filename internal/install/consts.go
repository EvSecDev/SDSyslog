package install

import "sdsyslog/internal/global"

const (
	DefaultSystemdUnitDir string = "/etc/systemd/system/"
	DefaultAAProfDir      string = "/etc/apparmor.d/"
	DefaultAAProfName     string = "usr.local.bin." + global.ProgBaseName
	DefaultPrivKeyPath    string = global.DefaultConfigDir + "/private.key"
)
