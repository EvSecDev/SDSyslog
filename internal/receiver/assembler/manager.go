// Manages assembler, shard, and deadline evaluator worker instances. Reassembles message fragments into original message order
package assembler

import (
	"context"
	"fmt"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Create new defrag manager
func (config *ManagerConfig) NewManager(ctx context.Context, outbox *mpmc.Queue[*protocol.Payload]) (new *Manager, err error) {
	// Double check queues - should never get past build
	if outbox == nil {
		panic("FATAL: Receiver Defrag manager received empty outbox queue variable")
	}

	ctx = logctx.AppendCtxTag(ctx, logctx.NSmDefrag)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	config.PacketDeadline.Store(global.DefaultMinPacketDeadline.Nanoseconds())

	err = config.validate()
	if err != nil {
		err = fmt.Errorf("invalid configuration: %w", err)
		return
	}

	startRouteSnapshot := routingSnapshot{
		instances: make(map[string]*Instance, config.MinInstanceCount.Load()),
		ids:       make([]string, config.MinInstanceCount.Load()),
	}

	new = &Manager{
		Config:   config,
		outQueue: outbox,
		ctx:      ctx,
	}

	new.routing.Store(&startRouteSnapshot)
	new.RoutingView = &RoutingState{
		manager: new,
	}
	return
}

// Checks manager configuration for invalid/missing values
func (config *ManagerConfig) validate() (err error) {
	if config.MinInstanceCount.Load() == 0 {
		err = fmt.Errorf("empty MinInstanceCount")
	}
	if config.MaxInstanceCount.Load() == 0 {
		err = fmt.Errorf("empty MaxInstanceCount")
	}
	if config.MinInstanceCount.Load() >= config.MaxInstanceCount.Load() {
		err = fmt.Errorf("minimum instance count cannot be equal to or less than max instance count")
	}
	if config.PacketDeadline.Load() == 0 {
		err = fmt.Errorf("empty packet deadline")
	}
	if config.FIPRSocketDirectory == "" {
		err = fmt.Errorf("empty socket directory")
	}
	return
}
