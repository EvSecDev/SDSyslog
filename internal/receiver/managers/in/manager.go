// Manages packet listener worker instances
package in

import (
	"context"
	"fmt"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
)

// Creates new instance manager
func (config *ManagerConfig) NewManager(ctx context.Context, dataOut *mpmc.Queue[listener.Container]) (new *Manager, err error) {
	ctx = logctx.AppendCtxTag(ctx, logctx.NSmIngest)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	config.replayCleanInterval = config.ReplayProtectionWindow / 2

	err = config.validate()
	if err != nil {
		err = fmt.Errorf("invalid configuration: %w", err)
		return
	}

	new = &Manager{
		Config:      config,
		Instances:   make(map[int]*Instance),
		outbox:      dataOut,
		replayCache: newReplayCacheWithShards(int(config.MaxInstanceCount.Load()), int64(config.ReplayProtectionWindow.Seconds())),
		ctx:         ctx,
	}

	// Background cleanup loop for replay protection cache
	go new.replayCache.cleanupLoop(ctx, new.Config.replayCleanInterval)

	return
}

// Checks manager configuration for invalid/missing values
func (config *ManagerConfig) validate() (err error) {
	if config.MinInstanceCount.Load() == 0 {
		err = fmt.Errorf("empty MaxQueueCapacity")
	}
	if config.MaxInstanceCount.Load() == 0 {
		err = fmt.Errorf("empty MaxQueueCapacity")
	}
	if config.MinInstanceCount.Load() >= config.MaxInstanceCount.Load() {
		err = fmt.Errorf("minimum instance count cannot be equal to or less than max instance count")
	}
	if config.Port == 0 {
		err = fmt.Errorf("empty listen port")
	}
	if config.ReplayProtectionWindow == 0 {
		err = fmt.Errorf("empty ReplayProtectionWindow")
	}
	if config.replayCleanInterval == 0 {
		err = fmt.Errorf("empty replayCleanInterval")
	}
	return
}
