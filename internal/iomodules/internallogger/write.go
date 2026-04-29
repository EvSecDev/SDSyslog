package internallogger

import (
	"fmt"
	"os"
	"sdsyslog/internal/logctx"
)

// Sender: Starts background writer to pull events from logger, format, and write to destination queue
func (injector *SenderInjector) Start() {
	eventChan := injector.logger.SetRawOutput()

	injector.wg.Go(func() {
		injector.run(eventChan)
	})

	injector.logger.UnsetFormattedOutput()
}

func (injector *SenderInjector) run(eventChan <-chan logctx.Event) {
	for {
		select {
		case event := <-eventChan:
			newMsg, err := loggerToProtocolMessage(event)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to format log event: %v", err)
				continue
			}
			err = injector.outbox.Push(newMsg, uint64(newMsg.Size()))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to push log event to queue: %v", err)
				continue
			}
		case <-injector.ctx.Done():
			return
		}
	}
}

// Stops internal log injector and removes raw output from logger itself
func (injector *SenderInjector) Stop() {
	if injector == nil {
		return
	}
	if injector.cancel != nil {
		injector.cancel()
	}
	injector.logger.UnsetRawOutput()
}

// Receiver: Starts background writer to pull events from logger, format, and write to destination queue
func (injector *ReceiverInjector) Start() {
	eventChan := injector.logger.SetRawOutput()

	injector.wg.Go(func() {
		injector.run(eventChan)
	})

	injector.logger.UnsetFormattedOutput()
}

func (injector *ReceiverInjector) run(eventChan <-chan logctx.Event) {
	for {
		select {
		case event := <-eventChan:
			newMsg, err := loggerToProtocolPayload(event, injector.hostID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to format log event: %v", err)
				continue
			}
			err = injector.outbox.Push(newMsg, uint64(newMsg.Size()))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to push log event to queue: %v", err)
				continue
			}
		case <-injector.ctx.Done():
			return
		}
	}
}

// Stops internal log injector and removes raw output from logger itself
func (injector *ReceiverInjector) Stop() {
	if injector == nil {
		return
	}
	if injector.cancel != nil {
		injector.cancel()
	}
	injector.logger.UnsetRawOutput()
}
