package assembler

import (
	"sdsyslog/internal/global"
	"sdsyslog/internal/queue/mpmc"
)

type Instance struct {
	Namespace      []string
	inbox          *mpmc.Queue[global.ParsedMessage] // messages from processors
	outbox         *mpmc.Queue[[]byte]               // fragments for sender
	hostID         int                               // ID for all sent messages
	maxPayloadSize int                               // maximum payload size for configured destination
	Metrics        MetricStorage
}
