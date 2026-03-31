package inotify

import (
	"context"
	"sync"
	"sync/atomic"
)

type Watcher struct {
	path     string
	fileName string

	// Kernel comms
	eventSize  uint32 // inotify event byte size
	instanceFD int    // inotify file descriptor
	wakeFD     int    // unblock syscall read on inotify event
	epollFD    int    // FD to contain inotify and wake fds

	// External comms
	fileHasChanged chan struct{} // File at current inode has changed contents
	fileHasRotated chan struct{} // File at path is not the same as the current inode

	// State
	fileFD atomic.Int32
	dirFD  atomic.Int32

	// Lifetime
	wg     sync.WaitGroup
	cancel context.CancelFunc // Stop instance
	ctx    context.Context
}
