// Manages assembler worker instances. Fragments log messages into pieces fitting within maximum payload size for network
package assembler

import (
	"context"
	"fmt"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/network"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Creates new instance manager
func (config *ManagerConfig) NewManager(ctx context.Context, outbox *mpmc.Queue[[]byte]) (new *Manager, err error) {
	// Double check queue - should never get past build
	if outbox == nil {
		panic("FATAL: Sender Packaging manager received empty outbox queue variable")
	}

	// Add log context
	ctx = logctx.AppendCtxTag(ctx, logctx.NSmPack)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	config.HostID, err = random.FourByte()
	if err != nil {
		err = fmt.Errorf("failed to generate new unique host identifier: %w", err)
		return
	}

	config.MaxPayloadSize, err = network.FindSendingMaxUDPPayload(config.DestinationIP)
	if err != nil {
		err = fmt.Errorf("failed to find max payload size: %w", err)
		return
	}
	if config.OverrideMaxPayloadSize != 0 {
		config.MaxPayloadSize = config.OverrideMaxPayloadSize
	}

	err = config.validate()
	if err != nil {
		err = fmt.Errorf("invalid configuration: %w", err)
		return
	}

	inbox, err := mpmc.New[protocol.Message](logctx.GetTagList(ctx),
		uint64(config.MinQueueCapacity),
		config.MinQueueCapacity,
		config.MaxQueueCapacity)
	if err != nil {
		return
	}

	startInstances := make([]*Instance, 0, config.MinInstanceCount.Load())

	new = &Manager{
		Config:   config,
		InQueue:  inbox,
		outQueue: outbox,
		ctx:      ctx,
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
	if config.MinQueueCapacity >= config.MaxQueueCapacity {
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
	if config.DestinationIP == "" {
		err = fmt.Errorf("empty destination ip")
	}
	if config.HostID == 0 {
		err = fmt.Errorf("empty host id")
	}
	if config.MaxPayloadSize == 0 {
		err = fmt.Errorf("empty max payload size")
	}
	if config.CryptoSuiteID == 0 {
		err = fmt.Errorf("uninitialized crypto suite ID")
	}

	return
}
