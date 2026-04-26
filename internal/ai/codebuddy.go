package ai

import "strings"

// codebuddyBackend is the CLIBackend instance for Codebuddy CLI.
var codebuddyBackend = &CLIBackend{
	name:           "codebuddy",
	defaultCommand: "codebuddy",
	buildArgs:      buildCodebuddyStreamArgs,
	newParser:      func() LineParser { return &StreamParser{} },
	filterLine: func(line string) (string, bool) {
		line = strings.TrimPrefix(line, "\xEF\xBB\xBF") // UTF-8 BOM
		if line == "" {
			return "", false
		}
		return line, true
	},
	preStart: nil,
}
