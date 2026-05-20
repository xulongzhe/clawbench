package version

// Version is set at build time via -ldflags "-X clawbench/internal/version.Version=...".
// When not set (e.g. bare "go build"), getBuildVersion() falls back to VCS info.
var Version = ""
