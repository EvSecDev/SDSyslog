// Package for sending received messages directly to desktop pop-up notifications (session DBUS freedesktop)
package dbusnotify

import (
	"fmt"
	"os"

	"github.com/godbus/dbus/v5"
)

// No-op. Always returns error
func NewInput() (module *InModule, err error) {
	err = fmt.Errorf("org.freedesktop.Notifications.Notify is not supported as an input")
	return
}

// Sets up new freedesktop notify send worker.
func NewOutput(outputEnabled bool) (module *OutModule, err error) {
	if !outputEnabled {
		// Comply with standard - module should be able to be called without wrapping decisions in caller
		return
	}

	module = &OutModule{}

	_, present := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS")
	if !present {
		err = fmt.Errorf("DBUS session address environment variable not present, refusing to start notification output")
		return
	}

	module.conn, err = dbus.ConnectSessionBus()
	if err != nil {
		err = fmt.Errorf("failed to connect to session DBUS: %w", err)
		return
	}

	path := dbus.ObjectPath(notifyPath)
	module.sink = module.conn.Object(notifyNamespace, path)
	return
}
