package dbusnotify

import (
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/iomodules"
	"sdsyslog/internal/iomodules/syslog"
	"sdsyslog/pkg/protocol"
	"strconv"

	"github.com/godbus/dbus/v5"
)

// Converts protocol fields to notification fields
func formatAsNotification(msg *protocol.Payload) (newNotification *notification, err error) {
	// Pull out custom fields from msg
	var appname, severity, facility, procID string
	var ok bool
	rawAppname, present := msg.CustomFields[iomodules.CFappname]
	if present {
		appname, ok = rawAppname.(string)
		if !ok {
			err = fmt.Errorf("failed to type assert application name to string")
			return
		}
	} else {
		appname = "-"
	}
	rawSeverity, present := msg.CustomFields[iomodules.CFseverity]
	if present {
		severity, ok = rawSeverity.(string)
		if !ok {
			err = fmt.Errorf("failed to type assert severity to string")
			return
		}
	} else {
		severity = iomodules.DefaultSeverity
	}
	rawFacility, present := msg.CustomFields[iomodules.CFfacility]
	if present {
		facility, ok = rawFacility.(string)
		if !ok {
			err = fmt.Errorf("failed to type assert facility to string")
			return
		}
	} else {
		facility = iomodules.DefaultFacility
	}
	rawProdID, present := msg.CustomFields[iomodules.CFprocessid]
	if present {
		procID, ok = rawProdID.(string)
		if !ok {
			err = fmt.Errorf("failed to type assert process ID to string")
			return
		}
	} else {
		procID = strconv.Itoa(os.Getpid())
	}

	// Convert syslog severities to notification urgency
	severityCode, err := syslog.SeverityToCode(severity)
	if err != nil {
		err = fmt.Errorf("failed to parse message severity: %w", err)
		return
	}
	var urgencyLevel int
	switch severityCode {
	case 0, 1, 2:
		// crit/alert/emerg
		urgencyLevel = highPriority
	case 3, 4:
		// warning/error
		urgencyLevel = normalPriority
	default:
		// notice/info/debug
		urgencyLevel = lowPriority
	}

	// Build notification details
	newNotification = &notification{
		appname:   appname + " (" + global.ProgBaseName + ")",
		replaceID: uint32(0), // no replace - always new notification
		icon:      "",        // No icon
		summary:   appname + "[" + procID + "]" + " - " + severity,
		body:      string(msg.Data),
		actions:   []string{}, // Buttons
		hints: map[string]dbus.Variant{
			"urgency":  dbus.MakeVariant(urgencyLevel),
			"category": dbus.MakeVariant(global.ProgBaseName + "." + facility + ".alert"),
		},
		popupDuration: int32(defaultPopupTime.Milliseconds()),
	}
	return
}
