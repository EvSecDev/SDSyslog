package shard

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sdsyslog/internal/crypto/hash"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/shard/fiprsend"
	"sdsyslog/pkg/protocol"
	"time"
)

// Route a fragment to a shard/process. Deterministic for all fragments of a message.
// Dynamically reroutes and tracks when targeted shard/process is shutdown.
func RouteFragment(ctx context.Context, rv RoutingView, remoteAddress string, fragment protocol.Payload, processingStartTime time.Time) (success bool) {
	var remoteShards []string
	if fragment.MessageSeqMax > 0 {
		// FIPR should only ever be used with fragmented messages
		var err error
		remoteShards, err = fiprsend.GetSocketFileList(global.DefaultSocketDir, os.Getpid())
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "%s\n", err.Error())
			return
		}
	}

	// One-off route decision retries
	// Not great, but I like this switch decision tree as is
retryRoute:

	// Identifier for all fragments within a given message per host
	bucketKey := fmt.Sprintf("%s-%d-%d", remoteAddress, fragment.HostID, fragment.MsgID)

	// Route Destination
	shardList := rv.GetAllIDs()
	if len(shardList) == 0 {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "no shards available\n")
		return
	}
	defaultIndex := hrwSelect(bucketKey, shardList)

	// Decision Tree - Send to a shard
	var routedDest, routedIndex string
	switch {
	case rv.BucketExists(defaultIndex, bucketKey):
		// Existing message - Send to default shard
		shard := rv.GetShard(defaultIndex)
		if shard == nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
				"shard ID %s disappeared while attempting to route fragment from message ID %d\n", defaultIndex, fragment.MsgID)
			return
		}
		shard.push(ctx, bucketKey, fragment, processingStartTime)

		routedDest = "shard"
		routedIndex = defaultIndex
	case len(remoteShards) > 0:
		// New (to local) Message - Remote shards available

		// Route Destination
		socketFile := hrwSelect(bucketKey, remoteShards)
		socketPath := filepath.Join(global.DefaultSocketDir, socketFile)

		// Fragments only get one chance to route remotely, otherwise they are forced local
		remoteShards = nil // Prevents endless loop

		rerouteLocal, err := fiprsend.RouteFragment(socketPath, bucketKey, remoteAddress, fragment)
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityData, global.ErrorLog,
				"failed to route message fragment to remote process (id '%s' will route to local): %w\n", bucketKey, err)
			goto retryRoute
		}
		if rerouteLocal {
			// At this point we know the fragment does not exist local and does not exist remote
			goto retryRoute
		}

		routedDest = "process"
		routedIndex = socketFile
	case rv.IsShardShutdown(defaultIndex):
		// New Message - Default shard is in shutdown - reroute to next highest weight
		nonDrainingIDs := rv.GetNonDrainingIDs()
		if len(nonDrainingIDs) == 0 {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "no active shards available\n")
			return
		}
		newIndex := hrwSelect(bucketKey, nonDrainingIDs)

		shard := rv.GetShard(newIndex)
		if shard == nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
				"shard ID %s disappeared while attempting to route fragment from message ID %d\n", defaultIndex, fragment.MsgID)
			return
		}
		shard.push(ctx, bucketKey, fragment, processingStartTime)

		routedDest = "shard"
		routedIndex = newIndex
	default:
		// New Message - Bucket not in shutdown
		shard := rv.GetShard(defaultIndex)
		if shard == nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
				"shard ID %s disappeared while attempting to route fragment from message ID %d\n", defaultIndex, fragment.MsgID)
			return
		}
		shard.push(ctx, bucketKey, fragment, processingStartTime)

		routedDest = "shard"
		routedIndex = defaultIndex
	}

	logctx.LogEvent(ctx, global.VerbosityData, global.InfoLog, "Sent message ID %d to %s %s\n", fragment.MsgID, routedDest, routedIndex)
	success = true
	return
}

func hrwSelect(key string, candidates []string) (selected string) {
	var maxScore uint64
	for i, id := range candidates {
		sum := hash.SHA256([]byte(key + id))
		score := binary.BigEndian.Uint64(sum[:8])

		if i == 0 || score > maxScore {
			maxScore = score
			selected = id
		}
	}
	return
}
