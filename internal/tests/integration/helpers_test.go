package integration

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"sdsyslog/internal/crypto/hash"
	"sdsyslog/internal/iomodules/generic"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver"
	"sdsyslog/internal/receiver/output"
	"sdsyslog/internal/sender"
	"strings"
	"time"
)

// Chose non-loopback interface for testing (smaller mtu than loopback)
func findLocalTestIP(ifaces []net.Interface) (testIp string) {
	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		var found bool
		for _, addr := range addrs {
			var ip net.IP

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Pick the first valid IPv4
			if ip != nil && ip.To4() != nil {
				testIp = ip.String()
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	return
}

// Uses logger in context to search logger buffer for events matching filter (must match all 3 filters if filters are not empty)
func filterLogBuffer(ctx context.Context, searchText, searchTag, searchSeverity string) (matches string, found bool) {
	logger := logctx.GetLogger(ctx)
	if logger == nil {
		return
	}

	lines := logger.GetFormattedLogLines()

	bracketRe := regexp.MustCompile(`\[[^\]]*\]`)
	var re *regexp.Regexp
	if searchTag != "" {
		// Match brackets containing the tag
		re = regexp.MustCompile(regexp.QuoteMeta(searchTag)) // just match the tag
	}

	var foundLines []string
	lastMsg := ""

	// Regex to strip the timestamp prefix [YYYY-MM-DDThh:mm:ss...]
	timestampRe := regexp.MustCompile(`^\[[^\]]*\]\s*`)

	for _, line := range lines {
		// Remove the timestamp for comparison
		msgOnly := timestampRe.ReplaceAllString(line, "")

		// Skip partial duplicates (same message ignoring timestamp)
		if msgOnly == lastMsg {
			continue
		}

		// Filter by tag if searchTag is non-empty
		if re != nil {
			// Extract all bracketed sections
			brackets := bracketRe.FindAllString(line, -1)
			foundTag := false
			for _, b := range brackets {
				if re.MatchString(b) {
					foundTag = true
					break
				}
			}
			if !foundTag {
				continue
			}
		}

		// Filter by severity if searchSeverity is non-empty
		if searchSeverity != "" && !strings.Contains(line, "["+searchSeverity+"]") {
			continue
		}

		// Filter by text if searchText is non-empty
		if searchText != "" && !strings.Contains(line, searchText) {
			continue
		}

		// Passed all filters, include line
		foundLines = append(foundLines, line)
		found = true
		lastMsg = msgOnly
	}

	messageCounts := make(map[string]int, len(foundLines))
	for _, foundLine := range foundLines {
		metaLastIndex := strings.LastIndex(foundLine, "]")
		lineLog := foundLine[metaLastIndex:]
		messageCounts[lineLog]++
	}

	var dedupFoundLines []string
	seenMessages := make(map[string]bool, len(foundLines))
	for _, foundLine := range foundLines {
		metaLastIndex := strings.LastIndex(foundLine, "] ")
		lineLog := foundLine[metaLastIndex:]

		if seenMessages[lineLog] {
			continue
		}
		seenMessages[lineLog] = true

		dedupLine := fmt.Sprintf("(repeated %d time(s)) %s", messageCounts[lineLog], foundLine)
		dedupFoundLines = append(dedupFoundLines, dedupLine)
	}

	matches = strings.Join(dedupFoundLines, "")
	return
}

// For watching for output of receiver pipeline
func waitForCompleteLines(testOutput *PipeBuffer, expected int, readMaxIdleTime time.Duration) (lineHashes [][]byte, err error) {
	reader := bufio.NewReaderSize(testOutput, 64*1024)

	rawLines := make([][]byte, 0, expected)

	for len(rawLines) < expected {
		var line []byte
		var readErr error

		done := make(chan struct{})
		go func() {
			line, readErr = reader.ReadBytes('\n')
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(readMaxIdleTime):
			err = fmt.Errorf("exceeded max line wait time before receiving %d lines (got %d)", expected, len(rawLines))
			return
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				// If EOF but we still got data without newline, ignore (incomplete line)
				if len(line) == 0 {
					err = fmt.Errorf("unexpected EOF before receiving %d lines (got %d)", expected, len(rawLines))
					return
				}

				// If EOF returned a partial line without newline, drop it
				if line[len(line)-1] != '\n' {
					err = fmt.Errorf("incomplete line at EOF")
					return
				}
			} else {
				err = fmt.Errorf("failed reading from pipe: %w", readErr)
				return
			}
		}

		// Ensure it's a complete line
		if len(line) == 0 || line[len(line)-1] != '\n' {
			continue
		}

		rawLines = append(rawLines, line)
	}

	for _, ln := range rawLines {
		var h []byte
		h, err = hash.MultipleSlices(ln)
		if err != nil {
			err = fmt.Errorf("hashing line: %w", err)
			return
		}

		lineHashes = append(lineHashes, h)
	}

	return
}

// Verifies metric collection is functional and counts are correct
func checkPipelineCounts(expectedCount int, startTime time.Time, senderDaemon *sender.Daemon, recvDaemon *receiver.Daemon, configuredPollInterval time.Duration) (err error) {
	// Bake for additional metric poll interval before search
	time.Sleep(configuredPollInterval)

	endTime := time.Now()

	// Send - Input
	var totalSendInCtn int
	sendInMetrics := senderDaemon.MetricDataSearcher(generic.MTBatchesRead, []string{logctx.NSSend, logctx.NSmIngest, logctx.NSoRaw}, startTime, endTime)
	for _, metric := range sendInMetrics {
		cnt, ok := metric.Value.Raw.(uint64)
		if !ok {
			err = fmt.Errorf("expected metric value to be uint64, but type assertion failed")
			return
		}
		totalSendInCtn += int(cnt)
	}
	if totalSendInCtn != expectedCount {
		err = fmt.Errorf("expected send input count to be %d, but got %d from metrics", expectedCount, totalSendInCtn)
		return
	}

	// Receive - Output
	var totalRecvOutCtn int
	recvOutMetrics := recvDaemon.MetricDataSearcher(output.MTRawWritesSuc, []string{logctx.NSRecv, logctx.NSmOutput}, startTime, endTime)
	for _, metric := range recvOutMetrics {
		cnt, ok := metric.Value.Raw.(uint64)
		if !ok {
			err = fmt.Errorf("expected metric value to be uint64, but type assertion failed")
			return
		}
		totalRecvOutCtn += int(cnt)
	}
	if totalSendInCtn != expectedCount {
		err = fmt.Errorf("expected receive output count to be %d, but got %d from metrics", expectedCount, totalSendInCtn)
		return
	}

	// Receive timeouts
	var totalBucketTimeoutsCtn int
	timeouts := recvDaemon.MetricDataSearcher("timed_out_buckets", []string{logctx.NSRecv, logctx.NSmDefrag}, startTime, endTime)
	for _, metric := range timeouts {
		cnt, ok := metric.Value.Raw.(uint64)
		if !ok {
			err = fmt.Errorf("expected metric value to be uint64, but type assertion failed")
			return
		}
		totalBucketTimeoutsCtn += int(cnt)
	}
	if totalBucketTimeoutsCtn > 0 {
		err = fmt.Errorf("expected receive bucket timeout count to be 0, but got %d from metrics", totalSendInCtn)
		return
	}

	// Bake for additional metric polling interval to separate tests
	time.Sleep(1 * configuredPollInterval)
	return
}
