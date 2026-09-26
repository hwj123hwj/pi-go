// Package appdir resolves the user's EasyAgent data directory and migrates the
// legacy pi-go directory on startup.
package appdir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hwj123hwj/easyagent/sdk/config"
)

const (
	currentDirName = config.HomeDirName
	legacyDirName  = ".pi-go"
)

// MigrateLegacyHome moves the old data directory as a whole when no new
// directory exists yet. It deliberately does not merge directories: a
// partially merged state could overwrite credentials or session history.
func MigrateLegacyHome() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return migrateLegacyPaths(filepath.Join(home, legacyDirName), config.HomeDir())
}

func migrateLegacyHome(home string) error {
	return migrateLegacyPaths(filepath.Join(home, legacyDirName), filepath.Join(home, currentDirName))
}

func migrateLegacyPaths(oldPath, newPath string) error {
	if oldPath == newPath {
		return nil
	}
	if _, err := os.Stat(oldPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("both legacy and current EasyAgent data directories exist (%s and %s); leaving both untouched", oldPath, newPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.Rename(oldPath, newPath)
}
