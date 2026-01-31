package assembler

import (
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

type Instance struct {
	Namespace      []string
	inbox          *mpmc.Queue[protocol.Message] // messages from processors
	outbox         *mpmc.Queue[[]byte]           // fragments for sender
	hostID         int                           // ID for all sent messages
	maxPayloadSize int                           // maximum payload size for configured destination
	Metrics        MetricStorage
}
