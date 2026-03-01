// Centralized package for any constants/variables/types that are used across the entire program but do not import anything
package global

import "time"

const (
	ProgVersion string  = "v0.15.6"
	SendMode    string  = "send"
	RecvMode    string  = "receive"
	CtxModeKey  CtxMode = "mode" // Identifying if program is in sender or receiver mode

	ProgBaseName             string        = "sdsyslog"
	DefaultBinaryPath        string        = "/usr/local/bin/" + ProgBaseName
	DefaultConfigDir         string        = "/etc/" + ProgBaseName
	DefaultConfigSend        string        = DefaultConfigDir + "/" + ProgBaseName + "-sender.json"
	DefaultConfigRecv        string        = DefaultConfigDir + "/" + ProgBaseName + ".json"
	DefaultStateDir          string        = "/var/cache/" + ProgBaseName
	DefaultStateFile         string        = DefaultStateDir + "/last.state"
	DefaultSocketDir         string        = DefaultStateDir + "/ipc"
	DefaultReceiverPort      int           = 8514
	DefaultMinQueueSize      int           = 512
	DefaultMaxQueueSize      int           = 4096
	DefaultMinPacketDeadline time.Duration = 200 * time.Millisecond // Also default starting deadline
	DefaultMaxPacketDeadline time.Duration = 1 * time.Second
)
