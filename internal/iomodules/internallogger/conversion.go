package internallogger

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/global"
	"sdsyslog/internal/iomodules"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"strings"
)

// Converts internal logger event to a protocol message
func loggerToProtocolMessage(event logctx.Event) (msg *protocol.Message, err error) {
	var syslogSeverity string
	switch event.Severity {
	case logctx.FatalLog:
		syslogSeverity = "crit"
	case logctx.ErrorLog:
		syslogSeverity = "err"
	case logctx.WarnLog:
		syslogSeverity = "warning"
	case logctx.InfoLog:
		syslogSeverity = "info"
	default:
		syslogSeverity = "debug"
	}

	hostname, err := os.Hostname()
	if err != nil {
		return
	}

	msg = &protocol.Message{
		Timestamp: event.Timestamp,
		Hostname:  hostname,
		Data:      []byte(event.Message),
		Fields: map[string]any{
			iomodules.CtxKey:      "Logctx",                      // Event produced by
			iomodules.CFnamespace: strings.Join(event.Tags, "/"), // Event created by
			iomodules.CFappname:   global.ProgBaseName,           // Always this program itself
			iomodules.CFprocessid: os.Getpid(),                   // Always this program itself
			iomodules.CFfacility:  iomodules.DefaultFacility,     // Only ever one facility
			iomodules.CFseverity:  syslogSeverity,                // Pass through
		},
	}
	return
}

// Converts internal logger event to a protocol payload
func loggerToProtocolPayload(event logctx.Event, hostID int) (payload *protocol.Payload, err error) {
	msg, err := loggerToProtocolMessage(event)
	if err != nil {
		return
	}

	newMessageID, err := random.FourByte()
	if err != nil {
		err = fmt.Errorf("failed to generate random message identifier: %w", err)
		return
	}

	simIP := net.ParseIP("::ffff:0:0:0:1")
	simAddr, ok := netip.AddrFromSlice(simIP)
	if !ok {
		err = fmt.Errorf("internal error: simulated IP is invalid")
		return
	}

	payload = &protocol.Payload{
		// Pass through from message creation
		Timestamp:    msg.Timestamp,
		Hostname:     msg.Hostname,
		CustomFields: msg.Fields,
		Data:         msg.Data,

		// Custom Added data
		RemoteIP:    simAddr,
		HostID:      hostID,
		MsgID:       newMessageID,
		SignatureID: 0,

		// Only for compatibility - not used
		MessageSeq:    0,
		MessageSeqMax: 0,
		PaddingLen:    23,
	}
	return
}
