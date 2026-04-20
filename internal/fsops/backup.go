package fsops

import (
	"fmt"
	"os"
	"time"
)

func MakeFileBackup(src string) (backupPath string, existed bool, err error) {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
			return
		}
		err = fmt.Errorf("stat source: %w", err)
		return
	}

	if info.IsDir() {
		err = fmt.Errorf("backup target is directory, not file")
		return
	}

	backupPath = fmt.Sprintf("%s.bak.%d.%d", src, os.Getpid(), time.Now().UnixNano())

	err = os.Rename(src, backupPath)
	if err != nil {
		err = fmt.Errorf("rename backup: %w", err)
		return
	}

	existed = true
	return
}
