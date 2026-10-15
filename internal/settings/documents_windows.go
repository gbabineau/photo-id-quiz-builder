//go:build windows

package settings

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// documentsDir follows Documents even when Windows or OneDrive has moved it.
func documentsDir() string {
	if dir, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0); err == nil && dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents")
}
