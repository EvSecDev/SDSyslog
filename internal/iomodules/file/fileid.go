package file

import (
	"fmt"
	"os"
	"syscall"
)

// Not supporting windows intentionally
// No cross platform way to retrieve inode-like id
// Build tags would require other parts of the program to also use build tags - I'm not doing that

// Retrieves unique file identifier
func getFileID(file os.FileInfo) (id fileID, err error) {
	stat, ok := file.Sys().(*syscall.Stat_t)
	if !ok {
		err = fmt.Errorf("unsupported stat type")
		return
	}
	id = fileID{
		dev: uint64(stat.Dev),
		ino: uint64(stat.Ino),
	}
	return
}
