package ai

// buildCodebuddyStreamArgs constructs the CLI arguments for Codebuddy streaming
func buildCodebuddyStreamArgs(req ChatRequest) []string {
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--include-partial-messages",
	}

	if req.Resume {
		args = append(args, "--resume", req.SessionID)
	} else {
		args = append(args, "--session-id", req.SessionID)
	}

	args = append(args, "--add-dir", req.WorkDir, "--dangerously-skip-permissions",
		"--disallowedTools", "CronCreate", "CronDelete", "CronList", "ToolSearch", "DeferExecuteTool")

	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	// Pass model name: per-request override takes priority
	modelName := req.Model
	if modelName == "" {
		// No model specified, use default from agent configuration
		// This should have been set by the caller based on agent config
		modelName = "glm-5.1"
	}
	if modelName != "" {
		args = append(args, "--model", modelName)
	}

	if req.Resume {
		// With --resume, prompt is read from stdin
	} else {
		// With --session-id, prompt is the last argument
		args = append(args, req.Prompt)
	}

	return args
}
