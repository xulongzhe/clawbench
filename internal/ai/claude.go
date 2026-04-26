package ai

import (
	"os/exec"
	"strings"
)

// claudeBackend is the CLIBackend instance for Claude CLI.
var claudeBackend = &CLIBackend{
	name:           "claude",
	defaultCommand: "claude",
	buildArgs:      buildClaudeStreamArgs,
	newParser:      func() LineParser { return &StreamParser{} },
	filterLine:     nil, // skip empty lines only (default)
	preStart: func(cmd *exec.Cmd, req ChatRequest) {
		if req.Resume {
			cmd.Stdin = strings.NewReader(req.Prompt)
		}
	},
}
