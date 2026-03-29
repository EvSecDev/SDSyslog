// File change/rotation notification worker utilizing Linux inotify for filesystem events
package inotify

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"sdsyslog/internal/logctx"

	"golang.org/x/sys/unix"
)

// Initializes new file watcher instance
func New(ctx context.Context, fileToWatch string) (new *Watcher, err error) {
	new = &Watcher{
		path:           fileToWatch,
		fileName:       filepath.Base(fileToWatch),
		fileHasChanged: make(chan struct{}, 1), // Main blocker for reading new lines
		fileHasRotated: make(chan struct{}, 1), // Notify when to switch file inode and reset offset
		eventSize:      uint32(unix.SizeofInotifyEvent),
	}
	new.ctx, new.cancel = context.WithCancel(ctx)

	// Open the inotify instance (non-blocking since we are using epoll)
	new.instanceFD, err = unix.InotifyInit1(unix.IN_NONBLOCK)
	if err != nil {
		err = fmt.Errorf("failed to initialize inotify: %w", err)
		return
	}

	// Use epoll to control unblocking of inotify blocking read (For shutdown draining)
	new.wakeFD, err = unix.Eventfd(0, unix.EFD_NONBLOCK)
	if err != nil {
		err = fmt.Errorf("failed to create eventfd: %w", err)
		return
	}
	new.epollFD, err = unix.EpollCreate1(0)
	if err != nil {
		err = fmt.Errorf("failed to create epoll: %w", err)
		return
	}
	err = unix.EpollCtl(new.epollFD, unix.EPOLL_CTL_ADD, new.instanceFD,
		&unix.EpollEvent{
			Events: unix.EPOLLIN | unix.EPOLLERR | unix.EPOLLHUP,
			Fd:     int32(new.instanceFD),
		})
	if err != nil {
		err = fmt.Errorf("failed to add inotify fd to epoll: %w", err)
		return
	}

	// Wake file descriptor for draining mode
	err = unix.EpollCtl(new.epollFD, unix.EPOLL_CTL_ADD, new.wakeFD,
		&unix.EpollEvent{
			Events: unix.EPOLLIN,
			Fd:     int32(new.wakeFD),
		})
	if err != nil {
		err = fmt.Errorf("failed to add wake fd to epoll: %w", err)
		return
	}

	// Add watcher for the log file
	watchDescriptorFile, err := unix.InotifyAddWatch(new.instanceFD,
		new.path,
		unix.IN_MODIFY|unix.IN_CLOSE_WRITE)
	if err != nil {
		err = fmt.Errorf("failed to add log file '%s' to inotify watcher: %w", new.path, err)
		return
	}
	new.fileFD.Store(int32(watchDescriptorFile))

	// Add watcher for the log dir
	logDirectory := filepath.Dir(new.path)
	watchDescriptorDir, err := unix.InotifyAddWatch(new.instanceFD,
		logDirectory,
		unix.IN_MOVED_TO|unix.IN_DELETE|unix.IN_CREATE)
	if err != nil {
		err = fmt.Errorf("failed to add directory '%s' to inotify watcher: %w", logDirectory, err)
		return
	}
	new.dirFD.Store(int32(watchDescriptorDir))

	return
}

// Spawns go routine for watcher worker
func (watcher *Watcher) Start() {
	watcher.wg.Add(1)
	go watcher.run()
}

// Supplies signals when file at inode changes
func (watcher *Watcher) FileChanged() <-chan struct{} {
	return watcher.fileHasChanged
}

// Supplies signals when inode is no longer at watched path (must reopen)
func (watcher *Watcher) FileRotated() <-chan struct{} {
	return watcher.fileHasRotated
}

// Gracefully stops watcher instance and cleans up file descriptors
func (watcher *Watcher) Stop() {
	watcher.cancel()

	// Wake epoll (unblock inotify blocked read)
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], 1)
	_, err := unix.Write(watcher.wakeFD, buf[:])
	if err != nil && !errors.Is(err, unix.EAGAIN) {
		logctx.LogStdWarn(watcher.ctx, "failed to write wakefd: %w\n", err)
	}

	watcher.wg.Wait()

	// Close inotify instance by file descriptor
	err = unix.Close(watcher.instanceFD)
	if err != nil {
		logctx.LogStdWarn(watcher.ctx, "failed to close inotify watcher file descriptor: %w\n", err)
	}
	err = unix.Close(watcher.wakeFD)
	if err != nil {
		logctx.LogStdWarn(watcher.ctx, "failed to close epoll wake file descriptor: %w\n", err)
	}
	err = unix.Close(watcher.epollFD)
	if err != nil {
		logctx.LogStdWarn(watcher.ctx, "failed to close epoll file descriptor: %w\n", err)
	}
}
