package dbusnotify

import "time"

const (
	notifyNamespace     string = "org.freedesktop.Notifications"
	notifyPath          string = "/org/freedesktop/Notifications"
	notifyCallNamespace string = "org.freedesktop.Notifications.Notify"

	defaultPopupTime time.Duration = 10 * time.Second

	lowPriority    int = 0
	normalPriority int = 1
	highPriority   int = 2
)
