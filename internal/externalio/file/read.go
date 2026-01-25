package file

import (
	"context"
	"io"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"syscall"
)

func (mod *InModule) Reader(ctx context.Context) {
	logFileInode, logFileOffset, err := getLastPosition(mod.filePath, mod.stateFile)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "failed to get position of last source file read for '%s': %v\n", mod.filePath, err)
	}

	_, err = mod.sink.Seek(logFileOffset, io.SeekStart)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to resume last source file read position for '%s': %v\n", mod.filePath, err)
	}

	var localHostname string
	var iter uint64
	const refreshMask = 1024 - 1
	localHostname, err = os.Hostname()
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "failed to retrieve current local hostname: %v\n", err)
		localHostname = "-"
		err = nil
	}

	// Create inotify background watcher
	fileHasChanged := make(chan bool, 1) // Main blocker for reading new lines
	fileHasRotated := make(chan bool, 1) // Notify when to switch file inode and reset offset
	go watcher(ctx, mod.filePath, fileHasChanged, fileHasRotated)

	buf := make([]byte, 65536)
	lineBuf := []byte{} // note: unbounded

	for {
		for {
			// Record current file position before read
			startOfChunk, err := mod.sink.Seek(0, io.SeekCurrent)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to retrieve current position in log file: %v\n", err)
			}

			n, err := mod.sink.Read(buf)
			if n == 0 {
				// no more bytes available, break to outer select for blocking
				break
			} else if err == io.EOF {
				// no more bytes available, break to outer select for blocking
				break
			} else if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "read error: %v", err)
				break
			}

			// process the bytes read
			offset := startOfChunk
			for i := 0; i < n; i++ {
				b := buf[i]

				if b != byte('\n') {
					// Regular line characters, add to buffer
					lineBuf = append(lineBuf, b)
					offset++
					continue
				}

				// line complete, process it
				mod.metrics.LinesRead.Add(1)

				msg := parseLine(string(lineBuf), localHostname)

				size := len(msg.Text) +
					len(msg.ApplicationName) +
					len(msg.Hostname) +
					len(msg.Facility) +
					len(msg.Severity) +
					16 // int64 size pid and time
				mod.outbox.PushBlocking(ctx, msg, size)
				mod.metrics.Success.Add(1)

				// reset line buffer
				lineBuf = []byte{}
				offset++ // move past newline
			}
			logFileOffset = offset

			// if line was complete, break inner loop to block
			if len(lineBuf) == 0 {
				break
			}
		}

		// Block until file change, file rotation, or cancellation
		select {
		case <-ctx.Done():
			err := savePosition(mod.stateFile, logFileInode, logFileOffset)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"failed to save position in file source '%s': %v\n", mod.filePath, err)
			}
			return
		case <-fileHasChanged:
			// file changed, continue scanning
		case reopenLogFile := <-fileHasRotated:
			if reopenLogFile {
				// Reopen at file path
				mod.sink.Close()
				mod.sink, err = os.Open(mod.filePath)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to reopen rotated log file: %v\n", err)
					continue
				}

				// Retrieve new file inode
				fileInfo, err := os.Stat(mod.filePath)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "unable to stat new source file: %v\n", err)
					continue
				}
				stat := fileInfo.Sys().(*syscall.Stat_t)

				// Save new file inode to state var
				logFileInode = stat.Ino

				// Reset offset position for new file
				logFileOffset = 0
			}
		}

		// Local hostname periodic refresh
		iter++
		if iter&refreshMask == 0 {
			newName, err := os.Hostname()
			if err == nil && newName != localHostname {
				localHostname = newName
			} else if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "failed to refresh current local hostname: %v\n", err)
			}
		}

		// Re-scan for new lines after the last offset
		_, err = mod.sink.Seek(logFileOffset, io.SeekStart)
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to seek to last offset: %v\n", err)
		}
	}

}
