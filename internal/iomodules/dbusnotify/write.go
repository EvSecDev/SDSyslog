package dbusnotify

import (
	"context"
	"fmt"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
)

func (mod *OutModule) Write(ctx context.Context, msg *protocol.Payload) (entriesWritten int, err error) {
	if mod == nil {
		return
	}

	notification, err := formatAsNotification(msg)
	if err != nil {
		err = fmt.Errorf("failed to format message as notification: %w", err)
		return
	}

	// https://specifications.freedesktop.org/notification/latest-single/
	// Example from cli
	/*
			gdbus call --session \
			    --dest org.freedesktop.Notifications \
				--object-path /org/freedesktop/Notifications \
		        --method org.freedesktop.Notifications.Notify \
		        my_app_name      # Type: string
		        0                # Type: uint32
				gtk-dialog-info  # Type: string
		        "The Summary"    # Type: string
				"Body of Notify" # Type: string
				[]               # Type: slice (string)
				{}               # Type: map
				5000             # Type: int32
	*/

	// Send notification (only order and type matters)
	notifyCall := mod.sink.CallWithContext(ctx, notifyCallNamespace, 0,
		notification.appname,
		notification.replaceID,
		notification.icon,
		notification.summary,
		notification.body,
		notification.actions,
		notification.hints,
		notification.popupDuration,
	)
	if notifyCall.Err != nil {
		err = fmt.Errorf("failed to send notification: %w", notifyCall.Err)
		return
	}

	var ret uint32
	lerr := notifyCall.Store(&ret)
	if lerr != nil {
		logctx.LogStdWarn(ctx, "failed retrieving uint32 return value: %w", lerr)
		return
	}

	entriesWritten = 1
	return
}

// No-op - satisfies common type
func (mod *OutModule) FlushBuffer() (flushedCnt int, err error) {
	return
}
