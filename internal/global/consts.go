// Centralized package for any constants/variables/types that are used across the entire program but do not import any other internals
package global

import "time"

const (
	ProgVersion string  = "v0.19.1"
	SendMode    string  = "send"
	RecvMode    string  = "receive"
	CtxModeKey  CtxMode = "mode" // Identifying if program is in sender or receiver mode

	ProgBaseName             string        = "sdsyslog"
	DefaultBinaryPath        string        = "/usr/local/bin/" + ProgBaseName
	DefaultConfigDir         string        = "/etc/" + ProgBaseName
	DefaultConfigSend        string        = DefaultConfigDir + "/" + ProgBaseName + "-sender.json"
	DefaultConfigRecv        string        = DefaultConfigDir + "/" + ProgBaseName + ".json"
	DefaultConfigPinKeys     string        = DefaultConfigDir + "/" + "pinned-sender-keys.json"
	DefaultSendSigningKey    string        = DefaultConfigDir + "/" + "sender-signer.key"
	DefaultStateDir          string        = "/var/cache/" + ProgBaseName
	DefaultStateFile         string        = DefaultStateDir + "/last.state"
	DefaultReceiverPort      int           = 8514
	DefaultMinQueueSize      MinValue      = 512
	DefaultMaxQueueSize      MaxValue      = 4096
	DefaultMinPacketDeadline time.Duration = 200 * time.Millisecond // Also default starting deadline
	DefaultMaxPacketDeadline time.Duration = 1 * time.Second
)
