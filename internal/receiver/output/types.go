package output

import (
	"sdsyslog/internal/externalio/beats"
	"sdsyslog/internal/externalio/file"
	"sdsyslog/internal/externalio/journald"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

type Instance struct {
	Namespace []string
	FileMod   *file.OutModule
	JrnlMod   *journald.OutModule
	BeatsMod  *beats.OutModule
	Inbox     *mpmc.Queue[protocol.Payload]
	Metrics   MetricStorage
}
