package inotify

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sdsyslog/internal/logctx"
	"time"

	"golang.org/x/sys/unix"
)

func (watcher *Watcher) run() {
	defer watcher.wg.Done()

	// Create a buffer to read the events
	buf := make([]byte, watcher.eventSize+8192)
	events := make([]unix.EpollEvent, 4)

	draining := false

	for {
		n, err := unix.EpollWait(watcher.epollFD, events, -1)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			logctx.LogStdErr(watcher.ctx, "epoll wait error: %v", err)
			return
		}

		for i := range n {
			fd := int(events[i].Fd)

			switch fd {
			case watcher.wakeFD:
				// Shutdown signal
				draining = true

				// drain wakeFD
				var tmp [8]byte
				for {
					_, err := unix.Read(watcher.wakeFD, tmp[:])
					if err != nil {
						if errors.Is(err, unix.EAGAIN) {
							break
						}
						logctx.LogStdWarn(watcher.ctx, "failed to read wake signal: %w\n", err)
						break
					}
				}
			case watcher.instanceFD:
				// inotify events
				for {
					n, err := unix.Read(watcher.instanceFD, buf)
					if err != nil {
						if errors.Is(err, unix.EAGAIN) {
							break // fully drained
						}
						logctx.LogStdErr(watcher.ctx, "read error: %v", err)
						break
					}

					if n == 0 {
						break
					}

					err = watcher.processEvent(buf, n)
					if err != nil {
						logctx.LogStdErr(watcher.ctx, "%w\n", err)
						continue
					}
				}
			}
		}

		// Exit only after full drain
		if draining {
			return
		}
	}
}

// Processes inotify event and sends signals if applicable
func (watcher *Watcher) processEvent(buf []byte, readEndIndex int) (err error) {
	var offset uint32
	for offset+watcher.eventSize <= uint32(readEndIndex) {
		// Retrieve the event
		var event unix.InotifyEvent
		eventBytes := buf[offset : offset+watcher.eventSize]
		reader := bytes.NewReader(eventBytes)
		err = binary.Read(reader, binary.LittleEndian, &event)
		if err != nil {
			err = fmt.Errorf("failed to read event content: %w", err)
			continue
		}

		// Nice booleans for event types
		isFileEvent := event.Wd == watcher.fileFD.Load()
		isDirEvent := event.Wd == watcher.dirFD.Load()
		isCloseWrite := event.Mask&unix.IN_CLOSE_WRITE != 0
		isModify := event.Mask&unix.IN_MODIFY != 0
		isCreate := event.Mask&unix.IN_CREATE != 0
		isMovedTo := event.Mask&unix.IN_MOVED_TO != 0

		// Contents at existing inode changed
		if isFileEvent && (isModify || isCloseWrite) {
			// Notify of change, but only when not consumed yet
			select {
			case watcher.fileHasChanged <- struct{}{}:
			default:
			}
		}

		// Name field has the filename for dir events (null-terminated)
		nameBytes := buf[offset+watcher.eventSize : offset+watcher.eventSize+uint32(event.Len)]
		nameBytes = bytes.TrimRight(nameBytes, "\x00")
		name := string(nameBytes)

		// Directory events for our file
		//   We only care about when something moved to our file or it was created
		if isDirEvent && name == watcher.fileName && (isCreate || isMovedTo) {
			// Cleanup watcher for old inode
			_, err = unix.InotifyRmWatch(watcher.instanceFD, uint32(watcher.fileFD.Load()))
			if err != nil && !errors.Is(err, unix.EINVAL) {
				logctx.LogStdWarn(watcher.ctx, "failed to remove previous inotify watcher for '%s': %w\n", watcher.path, err)
			}

			err = watcher.rotateInode()
			if err != nil {
				err = fmt.Errorf("failed to re-add rotated log file: %w", err)
			} else {
				// Notify of rotation, but only when not consumed yet
				select {
				case watcher.fileHasRotated <- struct{}{}:
				default:
				}
			}
		}

		// Move the offset forward to the next event
		offset += watcher.eventSize + uint32(event.Len)
	}
	return
}

// Attempts to swap file inotify watcher for new inode at given path
func (watcher *Watcher) rotateInode() (err error) {
	const maxRetries int = 5
	const maxDelay time.Duration = time.Minute
	delay := 100 * time.Millisecond

	for range maxRetries {
		// Attempt to add watcher for new inode
		var newFileFD int
		newFileFD, err = unix.InotifyAddWatch(watcher.instanceFD, watcher.path, unix.IN_MODIFY|unix.IN_CLOSE_WRITE)
		if err == nil {
			watcher.fileFD.Store(int32(newFileFD))
			return
		}

		// Errors not solved by waiting
		if !errors.Is(err, unix.EACCES) && !errors.Is(err, unix.EPERM) && !errors.Is(err, unix.ENOENT) {
			err = fmt.Errorf("failed to add rotated log file to inotify watcher: %w", err)
			return
		}

		// Wait and increment backoff
		time.Sleep(delay)

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	err = fmt.Errorf("failed to add rotated file to inotify watcher after %d retires within %.0f seconds: %w",
		maxRetries, maxDelay.Seconds(), err)
	return
}
