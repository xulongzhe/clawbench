package ai

// buildClaudeStreamArgs constructs the CLI arguments for Claude streaming
func buildClaudeStreamArgs(req ChatRequest) []string {
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
	}

	if req.Resume {
		args = append(args, "--resume", req.SessionID)
	} else {
		args = append(args, "--session-id", req.SessionID)
	}

	args = append(args, "--add-dir", req.WorkDir, "--dangerously-skip-permissions")

	// Disable built-in scheduling/timer tools to force use of ClawBench's
	// <schedule-proposal> mechanism instead of native CronCreate/CronDelete/CronList.
	args = append(args, "--disallowedTools", "CronCreate", "CronDelete", "CronList")

	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	// Pass model name if per-request override is set
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	if req.Resume {
		// With --resume, prompt is read from stdin
	} else {
		// With --session-id, prompt is the last argument
		args = append(args, req.Prompt)
	}

	return args
}
