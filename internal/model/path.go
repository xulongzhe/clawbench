package model

import (
	"path/filepath"
	"strings"
)

// ValidatePath validates that a relative path stays within the base directory boundary.
// Returns the resolved absolute path and whether it's valid.
func ValidatePath(basePath, relPath string) (string, bool) {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", false
	}
	fullPath := filepath.Join(absBase, relPath)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", false
	}
	valid := strings.HasPrefix(absPath, absBase+string(filepath.Separator)) || absPath == absBase
	return absPath, valid
}
