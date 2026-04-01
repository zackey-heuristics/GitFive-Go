package util

import (
	"os"
	"path/filepath"
)

// EnsureDir creates a directory (and parents) if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// ChangePermissions recursively sets owner rwx on all files and directories.
func ChangePermissions(path string) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chmod(p, 0o700)
	})
}

// DeleteTmpDir removes the gitfive temp directory at ~/.malfrats/gitfive/.tmp.
func DeleteTmpDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	tmpDir := filepath.Join(home, ".malfrats", "gitfive", ".tmp")
	if err := ChangePermissions(tmpDir); err != nil {
		// Directory might not exist, that's fine
		return nil
	}
	return os.RemoveAll(tmpDir)
}

// GitfiveDir returns the base gitfive config directory (~/.malfrats/gitfive).
func GitfiveDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".malfrats", "gitfive")
	if err := EnsureDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}
