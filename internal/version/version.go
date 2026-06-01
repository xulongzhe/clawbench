package version

import "runtime/debug"

// Version is set at build time via -ldflags "-X clawbench/internal/version.Version=...".
// When not set (e.g. bare "go build"), Get() falls back to VCS info.
var Version = ""

// Get returns a human-readable version string.
// Priority: ldflags-injected Version > VCS short SHA from build info > "dev".
func Get() string {
	if Version != "" {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			return s.Value[:7]
		}
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
