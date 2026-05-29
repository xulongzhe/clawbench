//go:build windows

package platform

import "os"

// listWindowsDrives enumerates available Windows drive letters by checking
// whether the root directory of each letter exists (A:\ through Z:\).
func listWindowsDrives() []string {
	var drives []string
	for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		root := string(letter) + ":\\"
		if _, err := os.Stat(root); err == nil {
			drives = append(drives, root)
		}
	}
	if len(drives) == 0 {
		// Fallback: at least return C:\
		drives = []string{"C:\\"}
	}
	return drives
}
