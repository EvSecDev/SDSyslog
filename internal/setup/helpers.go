package setup

import (
	"os"
	"path/filepath"
)

func restoreBkpFile(ctx *context, backupTgtPath, path string, installedFresh, replacedExisting bool) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	// Remove installed file if we installed one
	if installedFresh {
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			ctx.logger.Error("failed to remove target file path: %v", err)
		}
	}

	// Previous a file at target path, move back into place
	if replacedExisting {
		ctx.logger.Verbose("Restoring previous file from '%s' to '%s'", backupTgtPath, path)

		err := os.Rename(backupTgtPath, path)
		if err != nil {
			ctx.logger.Error("failed to restore backup: %v", err)
		} else {
			dir, err := os.Open(filepath.Dir(path))
			if err != nil {
				ctx.logger.Error("failed to open target file directory: %v", err)
			} else {
				err = dir.Sync()
				if err != nil {
					ctx.logger.Error("failed to sync target file directory: %v", err)
				}
				_ = dir.Close()
			}
		}
	}
}
