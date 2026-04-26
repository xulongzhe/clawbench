package ai

// geminiBackend is the CLIBackend instance for Gemini CLI.
var geminiBackend = &CLIBackend{
	name:           "gemini",
	defaultCommand: "gemini",
	buildArgs:      buildGeminiStreamArgs,
	newParser:      func() LineParser { return &GeminiStreamParser{} },
	filterLine:     filterSkipNonJSON(),
	preStart:       nil,
}
