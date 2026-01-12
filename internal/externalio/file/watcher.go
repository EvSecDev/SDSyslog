package file

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"path/filepath"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"strings"
	"syscall"
	"time"
)

func watcher(ctx context.Context, logFileInput string, fileHasChanged chan bool, fileHasRotated chan bool) {
	// Open the inotify instance
	fd, err := syscall.InotifyInit()
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to initialize inotify: %v\n", err)
		return
	}
	defer syscall.Close(fd)

	// Track active watcher fd's for dynamic cleanup
	watchDescriptors := make(map[string]int)
	defer func() {
		for _, descriptor := range watchDescriptors {
			syscall.InotifyRmWatch(fd, uint32(descriptor))
		}
	}()

	// Add watcher for the log file
	watchDescriptorFile, err := syscall.InotifyAddWatch(fd, logFileInput, syscall.IN_MODIFY|syscall.IN_CLOSE_WRITE)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to add log file '%s' to inotify watcher: %v\n", logFileInput, err)
		return
	}
	watchDescriptors["file"] = watchDescriptorFile

	// Add watcher for the log dir
	logDirectory := filepath.Dir(logFileInput)
	watchDescriptorDir, err := syscall.InotifyAddWatch(fd, logDirectory, syscall.IN_MOVED_FROM|syscall.IN_MOVED_TO|syscall.IN_DELETE|syscall.IN_CREATE)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to add directory '%s' to inotify watcher: %v\n", logDirectory, err)
		return
	}
	watchDescriptors["dir"] = watchDescriptorDir

	// Create a buffer to read the events
	buf := make([]byte, syscall.SizeofInotifyEvent+8192)
	logFileName := filepath.Base(logFileInput)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Read the event
			n, err := syscall.Read(fd, buf)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "error reading inotify event: %v\n", err)
				continue
			}

			var offset uint32
			for offset <= uint32(n)-syscall.SizeofInotifyEvent {
				var event syscall.InotifyEvent

				// Retrieve the event
				eventBytes := buf[offset : offset+syscall.SizeofInotifyEvent]
				reader := bytes.NewReader(eventBytes)
				err = binary.Read(reader, binary.LittleEndian, &event)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to read event content: %v\n", err)
					continue
				}

				// Name field has the filename for dir events (null-terminated)
				nameBytes := buf[offset+syscall.SizeofInotifyEvent : offset+syscall.SizeofInotifyEvent+uint32(event.Len)]
				name := string(nameBytes)
				name = strings.TrimRight(name, "\x00")

				// File modified
				if event.Mask&syscall.IN_MODIFY != 0 && event.Wd == int32(watchDescriptorFile) {
					fileHasChanged <- true
				}

				// Directory events - only look for our file
				if event.Wd == int32(watchDescriptorDir) && name == logFileName {
					if (event.Mask & (syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO | syscall.IN_DELETE | syscall.IN_CREATE)) != 0 {
						// Cleanup watcher for old inode
						syscall.InotifyRmWatch(fd, uint32(watchDescriptorFile))

						maxRetries := 5
						delay := 100 * time.Millisecond
						maxDelay := time.Minute

						var err error
						for range maxRetries {
							// Attempt to add watcher for new inode
							watchDescriptorFile, err = syscall.InotifyAddWatch(fd, logFileInput, syscall.IN_MODIFY|syscall.IN_CLOSE_WRITE)
							if err == nil {
								watchDescriptors["file"] = watchDescriptorFile // immediately add fd for cleanup just in case
								break
							}

							// Errors not solved by waiting
							if !errors.Is(err, syscall.EACCES) && !errors.Is(err, syscall.EPERM) && !errors.Is(err, syscall.ENOENT) {
								logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to add rotated log file to inotify watcher: %v\n", err)
								break
							}

							// Wait and increment backoff
							time.Sleep(delay)

							delay *= 2
							if delay > maxDelay {
								delay = maxDelay
							}
						}

						if err != nil {
							logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to add rotated log file to inotify watcher after %d retires within %.0f minutes: %v", maxRetries, maxDelay.Minutes(), err)
						} else {
							fileHasRotated <- true // send value to buffer so its available after main thread is unblocked
							fileHasChanged <- true // unblock main thread
						}
					}
				}

				// Move the offset forward to the next event
				offset += syscall.SizeofInotifyEvent + uint32(event.Len)
			}
		}
	}
}
