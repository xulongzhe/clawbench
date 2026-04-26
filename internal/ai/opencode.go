package ai

import "strings"

// opencodeBackend is the CLIBackend instance for OpenCode CLI.
var opencodeBackend = &CLIBackend{
	name:           "opencode",
	defaultCommand: "opencode",
	buildArgs:      buildOpenCodeStreamArgs,
	newParser:      func() LineParser { return &OpenCodeStreamParser{} },
	filterLine: func(line string) (string, bool) {
		if line == "" || strings.HasPrefix(line, "[opencode-mobile]") {
			return "", false
		}
		if !strings.HasPrefix(line, "{") {
			return "", false
		}
		return line, true
	},
	preStart: nil,
}
