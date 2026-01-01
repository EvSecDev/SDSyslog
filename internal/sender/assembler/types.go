package assembler

import (
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/sender/listener"
)

type Instance struct {
	Namespace      []string
	inbox          *mpmc.Queue[listener.ParsedMessage] // messages from processors
	outbox         *mpmc.Queue[[]byte]                 // fragments for sender
	hostID         int                                 // ID for all sent messages
	maxPayloadSize int                                 // maximum payload size for configured destination
	Metrics        *MetricStorage
}
