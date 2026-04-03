package dbusnotify

import "github.com/godbus/dbus/v5"

type OutModule struct {
	conn *dbus.Conn
	sink dbus.BusObject
}

type notification struct {
	appname       string
	replaceID     uint32
	icon          string
	summary       string
	body          string
	actions       []string
	hints         map[string]dbus.Variant
	popupDuration int32
}

type InModule struct{}
