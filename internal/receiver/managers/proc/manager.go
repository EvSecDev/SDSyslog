// Manages processor worker instances
package proc

import (
	"context"
	"fmt"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/shard"
)

// Creates new instance manager
func (config *ManagerConfig) NewManager(ctx context.Context, shardRouting shard.RoutingView) (new *Manager, err error) {
	err = config.validate()
	if err != nil {
		err = fmt.Errorf("invalid configuration: %w", err)
		return
	}

	// Add log context
	ctx = logctx.AppendCtxTag(ctx, logctx.NSProc)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	inQueue, err := mpmc.New[listener.Container](logctx.GetTagList(ctx),
		uint64(config.MinQueueCapacity),
		config.MinQueueCapacity,
		config.MaxQueueCapacity)
	if err != nil {
		return
	}

	new = &Manager{
		Config:      config,
		Instances:   make(map[int]*Instance),
		Inbox:       inQueue,
		routingView: shardRouting,
		ctx:         ctx,
	}
	return
}

// Checks manager configuration for invalid/missing values
func (config *ManagerConfig) validate() (err error) {
	if config.MinQueueCapacity == 0 {
		err = fmt.Errorf("empty MinQueueCapacity")
	}
	if config.MaxQueueCapacity == 0 {
		err = fmt.Errorf("empty MaxQueueCapacity")
	}
	if config.MinQueueCapacity >= config.MaxQueueCapacity {
		err = fmt.Errorf("minimum queue capacity cannot be equal to or less than max queue capacity")
	}
	if config.MinInstanceCount.Load() == 0 {
		err = fmt.Errorf("empty MaxQueueCapacity")
	}
	if config.MaxInstanceCount.Load() == 0 {
		err = fmt.Errorf("empty MaxQueueCapacity")
	}
	if config.MinInstanceCount.Load() >= config.MaxInstanceCount.Load() {
		err = fmt.Errorf("minimum instance count cannot be equal to or less than max instance count")
	}
	if config.PastMsgCutoff == 0 {
		err = fmt.Errorf("empty past message cutoff time")
	}
	if config.FutureMsgCutoff == 0 {
		err = fmt.Errorf("empty future message cutoff time")
	}

	return
}
