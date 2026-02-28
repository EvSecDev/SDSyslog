// Manages packet listener worker instances
package in

import (
	"context"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"time"
)

// Creates new instance manager
func NewInstanceManager(ctx context.Context,
	port int, outQueue *mpmc.Queue[listener.Container],
	minInsts, maxInsts int,
	replayProtectionWindow time.Duration) (new *InstanceManager) {

	ctx = logctx.AppendCtxTag(ctx, logctx.NSmIngest)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	new = &InstanceManager{
		Instances:           make(map[int]*Instance),
		MinInstCount:        minInsts,
		MaxInstCount:        maxInsts,
		port:                port,
		outbox:              outQueue,
		replayCleanInterval: replayProtectionWindow / 2,
		replayCache:         newReplayCacheWithShards(maxInsts, int64(replayProtectionWindow.Seconds())),
		ctx:                 ctx,
	}

	// Background cleanup loop for replay protection cache
	go new.replayCache.cleanupLoop(ctx, new.replayCleanInterval)

	return
}
