package shard

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/shard/fiprsend"
	"sdsyslog/pkg/protocol"
	"strconv"
	"strings"
	"time"
)

// Route a fragment to a shard/process. Deterministic for all fragments of a message.
// Dynamically reroutes and tracks when targeted shard/process is shutdown.
func RouteFragment(ctx context.Context, rv RoutingView, remoteAddress string, fragment protocol.Payload, processingStartTime time.Time) (success bool) {
	// Identifier for all fragments within a given message per host
	var b strings.Builder
	b.Grow(len(remoteAddress) + 32)
	b.WriteString(remoteAddress)
	b.WriteByte('-')
	b.WriteString(strconv.FormatInt(int64(fragment.HostID), 10))
	b.WriteByte('-')
	b.WriteString(strconv.FormatInt(int64(fragment.MsgID), 10))
	// Format Example: 127.0.0.1-1234-5678
	bucketKey := b.String()

	// Short circuit routing for single fragment messages
	if fragment.MessageSeqMax == 0 {
		nonDrainingIDs := rv.GetNonDrainingIDs()
		if len(nonDrainingIDs) == 0 {
			logctx.LogStdErr(ctx, "no active shards available\n")
			return
		}

		var shardIndex string
		if len(nonDrainingIDs) > 1 {
			shardIndex, _ = routeSelect(bucketKey, nonDrainingIDs)
		} else {
			shardIndex = nonDrainingIDs[0]
		}

		shard := rv.GetShard(shardIndex)
		if shard == nil {
			logctx.LogStdErr(ctx,
				"shard ID %s disappeared while attempting to route fragment from message ID %d\n", shardIndex, fragment.MsgID)
			return
		}
		shard.push(ctx, bucketKey, fragment, processingStartTime)
		logctx.LogEvent(ctx, logctx.VerbosityData, logctx.InfoLog, "Sent message ID %d to shard %s\n", fragment.MsgID, shardIndex)
		success = true
		return
	}

	var remoteShards []string
	if rv.IsFIPRRunning() {
		// FIPR should only ever be used with fragmented messages and when FIPR receiver is running
		var err error
		remoteShards, err = fiprsend.GetSocketFileList(rv.SocketDir(), os.Getpid())
		if err != nil {
			logctx.LogStdErr(ctx, "%s\n", err.Error())
			return
		}
	}

	const retryLimit int = 3
	for range retryLimit {
		shardList := rv.GetAllIDs()
		if len(shardList) == 0 {
			logctx.LogStdErr(ctx, "no shards available\n")
			return
		}

		primary, secondary := routeSelect(bucketKey, shardList)

		var routedDest, routedIndex string

		// Primary shard active
		if !rv.IsShardShutdown(primary) {
			shard := rv.GetShard(primary)
			if shard == nil {
				logctx.LogStdErr(ctx,
					"shard ID %s disappeared while attempting to route fragment from message ID %d\n",
					primary, fragment.MsgID)
				continue
			}

			shard.push(ctx, bucketKey, fragment, processingStartTime)

			routedDest = "shard"
			routedIndex = primary
			logctx.LogEvent(ctx, logctx.VerbosityData, logctx.InfoLog,
				"Sent message ID %d to %s %s\n", fragment.MsgID, routedDest, routedIndex)

			success = true
			return
		}

		// Primary shard draining but bucket already exists
		if rv.BucketExists(primary, bucketKey) {
			shard := rv.GetShard(primary)
			if shard == nil {
				logctx.LogStdErr(ctx,
					"shard ID %s disappeared while attempting to route fragment from message ID %d\n",
					primary, fragment.MsgID)
				continue
			}

			shard.push(ctx, bucketKey, fragment, processingStartTime)

			routedDest = "shard"
			routedIndex = primary
			logctx.LogEvent(ctx, logctx.VerbosityData, logctx.InfoLog,
				"Sent message ID %d to %s %s\n", fragment.MsgID, routedDest, routedIndex)

			success = true
			return
		}

		// Attempt remote routing for new messages
		if len(remoteShards) > 0 {
			primarySocket, _ := routeSelect(bucketKey, remoteShards)
			socketPath := filepath.Join(rv.SocketDir(), primarySocket)

			// prevent infinite retry
			remoteShards = nil

			rerouteLocal, err := fiprsend.RouteFragment(socketPath, bucketKey, remoteAddress, fragment)
			if err != nil {
				logctx.LogEvent(ctx, logctx.VerbosityData, logctx.ErrorLog,
					"failed to route message fragment to remote process (id '%s' will route to local): %w\n",
					bucketKey, err)
				continue
			}

			if rerouteLocal {
				// At this point we know the fragment does not exist local and does not exist remote
				continue
			}

			routedDest = "process"
			routedIndex = primarySocket

			logctx.LogEvent(ctx, logctx.VerbosityData, logctx.InfoLog,
				"Sent message ID %d to %s %s\n", fragment.MsgID, routedDest, routedIndex)

			success = true
			return
		}

		// Primary shard draining, fallback
		if secondary == "" {
			logctx.LogStdErr(ctx, "no fallback shard available\n")
			return
		}

		shard := rv.GetShard(secondary)
		if shard == nil {
			logctx.LogStdErr(ctx,
				"fallback shard ID %s disappeared while attempting to route fragment from message ID %d\n",
				secondary, fragment.MsgID)
			continue
		}

		shard.push(ctx, bucketKey, fragment, processingStartTime)

		routedDest = "shard"
		routedIndex = secondary

		logctx.LogEvent(ctx, logctx.VerbosityData, logctx.InfoLog,
			"Sent message ID %d to %s %s\n", fragment.MsgID, routedDest, routedIndex)

		success = true
		return
	}

	// Hit limit
	logctx.LogEvent(ctx, logctx.VerbosityData, logctx.ErrorLog,
		"Dropped message ID %d: could not route to any shard and hit route retry limit\n", fragment.MsgID)
	success = false
	return
}

// Load balancing selection providing a primary and backup selected candidate.
// Routing algorithm is modulo hash selection.
func routeSelect(key string, candidates []string) (primary, secondary string) {
	candidateNum := len(candidates)
	if candidateNum == 0 {
		return
	}

	if candidateNum == 1 {
		primary = candidates[0]
		secondary = candidates[0]
		return
	}

	// Hash the key (deterministic, uniform)
	sum := sha256.Sum256([]byte(key))
	h := binary.BigEndian.Uint64(sum[:8]) % uint64(candidateNum)

	// Map to primary index
	idx := int(h % uint64(candidateNum))
	primary = candidates[idx]

	// Deterministic secondary fallback: pick the next candidate (wrap around)
	secondary = candidates[(idx+1)%candidateNum]
	return
}
