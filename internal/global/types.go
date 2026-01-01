package global

type CommandSet struct {
	CommandName     string                 // Exact name of cli command
	UsageOption     string                 // Expected command value in usage top line
	Description     string                 // Short text displayed on parent command
	FullDescription string                 // Long text displayed on current command
	ChildCommands   map[string]*CommandSet // Available subcommands
}

type CtxKey string

// Receiving Daemon

type RecvConfig struct {
	PublicKey  string         `json:"publicKey"`
	PrivateKey string         `json:"privateKey"`
	Network    RecvNetwork    `json:"network"`
	Outputs    RecvOutputs    `json:"outputs"`
	Metrics    MetricConf     `json:"metrics"`
	Protocol   ProtocolConfig `json:"protocol"`
}

type RecvNetwork struct {
	Listeners []struct {
		Address   string `json:"address,omitempty"`
		Interface string `json:"interface,omitempty"`
		Port      int    `json:"port"`
	} `json:"listeners"`
}

type RecvOutputs struct {
	FilePath string `json:"filePath"`
	Journald bool   `json:"journald"`
	Stdout   bool   `json:"stdout"`
}

// Sending Daemon

type SendConfig struct {
	PublicKey string     `json:"publicKey"`
	Network   SNetConf   `json:"network"`
	Metrics   MetricConf `json:"metrics"`
}

type SNetConf struct {
	Destinations []struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
	} `json:"destinations"`
	MTU int `json:"mtu"`
}

type SendInputs struct {
	FilePath string      `json:"filePath,omitempty"`
	Journald bool        `json:"journald"`
	Stdin    bool        `json:"stdin"`
	Syslog   RecvNetwork `json:"syslog,omitempty"`
}

// Both Configs

type ProtocolConfig struct {
	PacketDeadline string `json:"packetDeadline"`
}

type MetricConf struct {
	Enabled          bool     `json:"enabled"`
	PollingIntervals []string `json:"pollingIntervals"`
	RetentionPeriod  string   `json:"inMemoryRetentionPeriod"`
	ExternalAccess   bool     `json:"externalAccessEnabled"`
}

type Logging struct {
	Level   int    `json:"logLevel"`
	LogFile string `json:"logFile,omitempty"`
}
