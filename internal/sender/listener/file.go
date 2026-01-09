// Watches and reads lines from configured sources
package listener

import (
	"context"
	"io"
	"os"
	"sdsyslog/internal/externalio/file"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"syscall"
)

// New creates a file listener instance
func NewFileSource(namespace []string, filePath string, stateFilePath string, queue *mpmc.Queue[global.ParsedMessage]) (new *FileInstance) {
	new = &FileInstance{
		Namespace:       append(namespace, global.NSoFile),
		Outbox:          queue,
		SourceFilePath:  filePath,
		SourceStateFile: stateFilePath,
		Metrics:         &MetricStorage{},
	}
	return
}

func (instance *FileInstance) Run(ctx context.Context) {
	logFileInode, logFileOffset, err := file.GetLastPosition(instance.SourceFilePath, instance.SourceStateFile)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "failed to get position of last source file read for '%s': %v\n", instance.SourceFilePath, err)
	}

	_, err = instance.Source.Seek(logFileOffset, io.SeekStart)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to resume last source file read position for '%s': %v\n", instance.SourceFilePath, err)
	}

	// Create inotify background watcher
	fileHasChanged := make(chan bool, 1) // Main blocker for reading new lines
	fileHasRotated := make(chan bool, 1) // Notify when to switch file inode and reset offset
	go file.Watcher(ctx, instance.SourceFilePath, fileHasChanged, fileHasRotated)

	buf := make([]byte, 65536)
	lineBuf := []byte{} // note: unbounded

	for {
		for {
			// Record current file position before read
			startOfChunk, err := instance.Source.Seek(0, io.SeekCurrent)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to retrieve current position in log file: %v\n", err)
			}

			n, err := instance.Source.Read(buf)
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
				instance.Metrics.LinesRead.Add(1)

				msg := file.ParseLine(string(lineBuf))

				size := len(msg.Text) +
					len(msg.ApplicationName) +
					len(msg.Hostname) +
					len(msg.Facility) +
					len(msg.Severity) +
					16 // int64 size pid and time
				instance.Outbox.PushBlocking(ctx, msg, size)
				instance.Metrics.Success.Add(1)

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
			err := file.SavePosition(instance.SourceStateFile, logFileInode, logFileOffset)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"failed to save position in file source '%s': %v\n", instance.SourceFilePath, err)
			}
			return
		case <-fileHasChanged:
			// file changed, continue scanning
		case reopenLogFile := <-fileHasRotated:
			if reopenLogFile {
				// Reopen at file path
				instance.Source.Close()
				instance.Source, err = os.Open(instance.SourceFilePath)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to reopen rotated log file: %v\n", err)
					continue
				}

				// Retrieve new file inode
				fileInfo, err := os.Stat(instance.SourceFilePath)
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

		// Re-scan for new lines after the last offset
		_, err = instance.Source.Seek(logFileOffset, io.SeekStart)
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to seek to last offset: %v\n", err)
		}
	}

}
