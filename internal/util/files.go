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

// gitfiveDirName is the on-disk directory name (under $HOME) used for
// credentials, sessions, and per-target temp data.
const gitfiveDirName = ".gitfive_go"

// DeleteTmpDir removes the gitfive temp directory at ~/.gitfive_go/.tmp.
func DeleteTmpDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	tmpDir := filepath.Join(home, gitfiveDirName, ".tmp")
	if err := ChangePermissions(tmpDir); err != nil {
		// Directory might not exist, that's fine
		return nil
	}
	return os.RemoveAll(tmpDir)
}

// GitfiveDir returns the base gitfive config directory (~/.gitfive_go).
func GitfiveDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, gitfiveDirName)
	if err := EnsureDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}
