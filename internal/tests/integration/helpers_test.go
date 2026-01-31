package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"sdsyslog/internal/crypto/hash"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver"
	"sdsyslog/internal/sender"
	"sdsyslog/internal/syslog"
	"sdsyslog/pkg/protocol"
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

	matches = strings.Join(foundLines, "")
	return
}

// For watching for output of receiver pipeline
func waitForCompleteLines(f *os.File, expected int) (lineHashes [][]byte, err error) {
	deadline := time.Now().Add(10 * time.Second) // Default timeout

	var (
		lastSize    int64 = -1
		stableSince time.Time
	)

	for {
		if time.Now().After(deadline) {
			err = fmt.Errorf("timeout waiting for %d complete lines", expected)
			return
		}

		var info os.FileInfo
		info, err = f.Stat()
		if err != nil {
			return
		}

		curSize := info.Size()

		if curSize != lastSize {
			// file changed
			lastSize = curSize
			stableSince = time.Now()
		}

		// read whole file
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return
		}

		var data []byte
		data, err = io.ReadAll(f)
		if err != nil {
			return
		}

		// split lines
		rawLines := bytes.Split(data, []byte("\n"))

		// discard incomplete final line
		if len(rawLines) > 0 && len(rawLines[len(rawLines)-1]) == 0 {
			rawLines = rawLines[:len(rawLines)-1]
		}

		// are there enough complete lines?
		if len(rawLines) >= expected {
			// Has file been quiet long enough?
			if time.Since(stableSince) >= 150*time.Millisecond {
				// Finalize hashing and return
				for _, ln := range rawLines {
					// Retrieve only data (omitting metadata from raw text output)
					var data string
					data, err = getDataFromFullLog(string(ln))
					if err != nil {
						err = fmt.Errorf("failed separating metadata: %w", err)
						return
					}

					var h []byte
					h, err = hash.MultipleSlices(append([]byte(data), '\n'))
					if err != nil {
						err = fmt.Errorf("hashing line: %w", err)
						return
					}
					lineHashes = append(lineHashes, h)
				}
				return
			}
		}

		// otherwise wait and retry
		time.Sleep(2 * time.Millisecond)
	}
}

func getDataFromFullLog(line string) (data string, err error) {
	const sep string = "]:"

	// find first occurrence
	i1 := strings.Index(line, sep)
	if i1 == -1 {
		err = fmt.Errorf("unknown log format, unable to find known delimiter ']:'")
		return
	}

	// find second occurrence (search after the first)
	i2 := strings.Index(line[i1+len(sep):], sep)
	if i2 == -1 {
		err = fmt.Errorf("unknown log format, unable to find known second delimiter ']:'")
		return
	}

	// convert second index to absolute position
	i2 = i1 + len(sep) + i2

	// Second half is data
	data = line[i2+len(sep):]
	data = strings.TrimSpace(data)
	return
}

// Verifies metric collection is functional and counts are correct
func checkPipelineCounts(expectedCount int, startTime time.Time, senderDaemon *sender.Daemon, recvDaemon *receiver.Daemon, configuredPollInterval time.Duration) (err error) {
	// Bake for additional metric poll interval before search
	time.Sleep(2 * configuredPollInterval)

	endTime := time.Now()

	// Send - Input
	var totalSendInCtn int
	sendInMetrics := senderDaemon.MetricDataSearcher("lines_read", []string{global.NSSend, global.NSmIngest}, startTime, endTime)
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
	recvOutMetrics := recvDaemon.MetricDataSearcher("success_writes", []string{global.NSRecv, global.NSmOutput}, startTime, endTime)
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
	timeouts := recvDaemon.MetricDataSearcher("timed_out_buckets", []string{global.NSRecv, global.NSmDefrag}, startTime, endTime)
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

// Creates a repeated string targeting desired length
func mockMessage(seedText string, targetPktSizeBytes int) (messageText string, err error) {
	mockLen := len(seedText)
	if mockLen > targetPktSizeBytes {
		err = fmt.Errorf("cannot create mock packets with individual sizes of %d bytes if the mock content is only %d bytes", targetPktSizeBytes, mockLen)
		return
	}

	// Repeat target message to approach targeted size
	msgRepetition := targetPktSizeBytes / mockLen
	messageText = strings.Repeat(seedText, msgRepetition)
	return
}

// Creates set number of packets with desired content (attempts to hit target size, but not exact)
func mockPackets(numMessages int, rawMessage string, maxPayloadSize int, publicKey []byte) (packets [][]byte, err error) {
	if numMessages == 0 {
		err = fmt.Errorf("cannot create mock packets if requested number of packets is 0")
		return
	}

	// Pre-startup
	syslog.InitBidiMaps()
	wrappers.SetupEncryptInnerPayload(publicKey)

	mainHostID, err := random.FourByte()
	if err != nil {
		err = fmt.Errorf("failed to generate new unique host identifier: %v", err)
		return
	}

	fields := map[string]any{
		"Facility":        22,
		"Severity":        5,
		"ProcessID":       3483,
		"ApplicationName": "test-app",
	}

	newMsg := protocol.Message{
		Timestamp: time.Now(),
		Hostname:  "localhost",
		Fields:    fields,
		Data:      rawMessage,
	}

	for range numMessages {
		var fragments [][]byte
		fragments, err = protocol.Create(newMsg, mainHostID, maxPayloadSize)
		if err != nil {
			err = fmt.Errorf("failed serialize test data for mock packets: %v", err)
			return
		}
		packets = append(packets, fragments...)
	}

	return
}
