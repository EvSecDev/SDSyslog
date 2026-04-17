// Manages output writer worker instance. Handles writing final fragmented log messages to configured network destinations
package output

import (
	"context"
	"fmt"
	"net"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
)

// Creates new instance manager (and its own inbox)
func (config *ManagerConfig) NewManager(ctx context.Context) (new *Manager, err error) {
	err = config.validate()
	if err != nil {
		err = fmt.Errorf("invalid configuration: %w", err)
		return
	}

	// Add log context
	ctx = logctx.AppendCtxTag(ctx, logctx.NSmOutput)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	// Setup destination network connection
	destinationConnection, err := net.DialUDP("udp", config.SourceAddress, config.DestAddress)
	if err != nil {
		err = fmt.Errorf("failed to open udp socket: %w", err)
		return
	}

	// Setup input queue
	inQueue, err := mpmc.New[[]byte](logctx.GetTagList(ctx),
		uint64(config.MinQueueCapacity),
		config.MinQueueCapacity,
		config.MaxQueueCapacity)
	if err != nil {
		return
	}

	startInstances := make([]*Instance, 0, config.MinInstanceCount.Load())

	new = &Manager{
		Config:  config,
		InQueue: inQueue,
		outDest: destinationConnection,
		ctx:     ctx,
	}
	new.Instances.Store(&startInstances)
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
	if int(config.MinQueueCapacity) >= int(config.MaxQueueCapacity) {
		err = fmt.Errorf("minimum queue capacity cannot be equal to or less than max queue capacity")
	}
	if config.MinInstanceCount.Load() == 0 {
		err = fmt.Errorf("empty MinQueueCapacity")
	}
	if config.MaxInstanceCount.Load() == 0 {
		err = fmt.Errorf("empty MaxQueueCapacity")
	}
	if config.MinInstanceCount.Load() >= config.MaxInstanceCount.Load() {
		err = fmt.Errorf("minimum instance count cannot be equal to or less than max instance count")
	}
	if config.SourceAddress == nil {
		err = fmt.Errorf("empty source address")
	}
	if config.DestAddress == nil {
		err = fmt.Errorf("empty destination address")
	}
	return
}
