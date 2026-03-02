// Manages output writer worker instance. Handles writing final assembled log messages to configured output destinations (file, journald, ect.)
package output

import (
	"context"
	"fmt"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Creates new instance manager with shared queue (between assemblers and output workers)
func (config *ManagerConfig) NewManager(ctx context.Context) (new *Manager, err error) {
	err = config.validate()
	if err != nil {
		err = fmt.Errorf("invalid configuration: %w", err)
		return
	}

	// Add log context
	ctx = logctx.AppendCtxTag(ctx, logctx.NSmOutput)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	inbox, err := mpmc.New[protocol.Payload](logctx.GetTagList(ctx),
		uint64(config.MinQueueCapacity),
		config.MinQueueCapacity,
		config.MaxQueueCapacity)
	if err != nil {
		return
	}

	new = &Manager{
		Config: config,
		Inbox:  inbox,
		ctx:    ctx,
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
	return
}
