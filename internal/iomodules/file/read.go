package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sdsyslog/internal/iomodules"
	"sdsyslog/internal/logctx"
	"strings"
)

// Watches file path for changes and rotations (truncation or move/create).
// Does NOT guarantee data integrity of intermediate files when rotated rapidly (2+ in sub-millisecond intervals).
func (mod *InModule) reader() {
	defer mod.wg.Done()
	ctx := mod.ctx

	// For hostname periodic refresh
	var iter uint64
	const refreshMask = 1024 - 1

	mod.watcher.Start()

	buf := make([]byte, 65536)
	lineBuf := []byte{} // note: unbounded per line (reset at line completion)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		func() {
			// Record panics and continue processing
			defer func() {
				if fatalError := recover(); fatalError != nil {
					stack := debug.Stack()
					logctx.LogStdErr(ctx,
						"panic in file ingest module thread: %v\n%s", fatalError, stack)
				}
			}()

			// Read all new file data since last read
			err := mod.fileReadAll(ctx, &lineBuf, buf)
			if err != nil {
				logctx.LogStdErr(ctx, "%w\n", err)
			}

			// Block until file change, file rotation, or cancellation
			select {
			case <-ctx.Done():
				mod.watcher.Stop()
				err = savePosition(mod.stateFile, mod.currentReadID.ino, mod.currentReadOffset)
				if err != nil {
					logctx.LogStdErr(ctx,
						"failed to save position in file source '%s': %w\n", mod.filePath, err)
				}
				return
			case <-mod.watcher.FileChanged():
				// file changed, continue scanning
			case <-mod.watcher.FileRotated():
				err = mod.fileReadAll(ctx, &lineBuf, buf)
				if err != nil {
					logctx.LogStdErr(ctx,
						"error reading pre-rotation file: %w\n", err)
				} else {
					err = mod.reopenLogfile(ctx)
					if err != nil {
						logctx.LogStdErr(ctx, "%w\n", err)
					}
				}
			}

			// Check for truncation
			file, err := mod.sink.Stat()
			if err != nil {
				logctx.LogStdWarn(ctx, "failed to stat current tracked file: %w\n", err)
				// Can't do anything about the error here, just read from beginning
				mod.currentReadOffset = 0
				lineBuf = lineBuf[:0]
			}
			if mod.currentReadOffset > file.Size() {
				// Truncation detected - reset state and seek to beginning
				logctx.LogStdWarn(ctx, "file '%s' has been truncated, seeking to start of file (warning: late writes to file might be missed)\n",
					mod.filePath)
				mod.currentReadOffset = 0
				lineBuf = lineBuf[:0]
			}

			// Local hostname periodic refresh
			iter++
			if iter&refreshMask == 0 {
				newName, err := os.Hostname()
				if err == nil && newName != mod.localHostname {
					mod.localHostname = newName
				} else if err != nil {
					logctx.LogStdWarn(ctx,
						"failed to refresh current local hostname: %w\n", err)
				}
			}

			// Re-scan for new lines after the last offset
			_, err = mod.sink.Seek(mod.currentReadOffset, io.SeekStart)
			if err != nil {
				logctx.LogStdErr(ctx,
					"failed to seek to last offset: %w\n", err)
			}
		}()
	}
}

// Reads file lines continuously until 0 bytes left or EOF in file
func (mod *InModule) fileReadAll(ctx context.Context, lineBuf *[]byte, buf []byte) (err error) {
	for {
		// Record current file position before read
		mod.currentReadOffset, err = mod.sink.Seek(0, io.SeekCurrent)
		if err != nil {
			err = fmt.Errorf("failed to retrieve current position in log file: %w", err)
			return
		}

		var n int
		n, err = mod.sink.Read(buf)
		if n == 0 || err == io.EOF {
			// no more bytes available, break to outer select for blocking
			err = nil
			break
		} else if err != nil {
			err = fmt.Errorf("read error: %w", err)
			break
		}

		// process the bytes read
		mod.processFileChunk(ctx, lineBuf, buf[:n])
	}
	return
}

// Steps through raw data from file and extracts lines
func (mod *InModule) processFileChunk(ctx context.Context, lineBuf *[]byte, buf []byte) {
	for i := 0; i < len(buf); i++ {
		char := buf[i]

		if char != '\n' {
			// Regular line characters, add to buffer
			*lineBuf = append(*lineBuf, char)
			mod.currentReadOffset++
			continue
		}

		// line complete, process it
		mod.metrics.LinesRead.Add(1)

		msg := parseLine(string(*lineBuf), mod.localHostname)

		msg.Fields[iomodules.CtxKey] = strings.Join(logctx.GetTagList(ctx), "/")

		var dropMsg bool
		for _, filter := range mod.filters {
			dropMsg = filter.Match(msg)
			if dropMsg {
				// First filter match wins
				break
			}
		}

		if !dropMsg {
			mod.outbox.PushBlocking(ctx, msg, msg.Size())
			mod.metrics.Success.Add(1)
		}

		// reset line buffer
		*lineBuf = (*lineBuf)[:0] // alter original to remove contents
		mod.currentReadOffset++   // move past newline
	}
}
