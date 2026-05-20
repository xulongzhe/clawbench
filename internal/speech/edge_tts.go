package speech

const (
	// edgeDefaultVoice is the default Chinese voice for edge-tts.
	edgeDefaultVoice = "zh-CN-XiaoxiaoNeural"
)

// EdgeTTSProvider implements SpeechProvider using edge-tts (Microsoft Edge TTS).
// edge-tts is free, has no quota limits, and provides high-quality Chinese voices.
type EdgeTTSProvider struct {
	CLISpeechProvider
	// Voice is the edge-tts voice ID (default: "zh-CN-XiaoxiaoNeural").
	Voice string
	// Rate is the speech speed adjustment (e.g. "+0%", "+20%", "-10%").
	Rate string
}

// NewEdgeTTSProvider creates an EdgeTTSProvider with sensible defaults.
func NewEdgeTTSProvider() *EdgeTTSProvider {
	p := &EdgeTTSProvider{
		Voice: edgeDefaultVoice,
		Rate:  "+0%",
	}

	p.CLISpeechProvider = newCLISpeechProvider(SynthesizeOptions{
		RelativePath: ".venv/bin/edge-tts",
		TextSource:   TextViaTempFile,
		LogName:      "edge-tts",
		ExtraArgs: func(cliPath string, text string, outputPath string, _ string) []string {
			args := []string{
				"--voice", p.Voice,
				"--file", text,
				"--write-media", outputPath,
			}
			if p.Rate != "" && p.Rate != "+0%" {
				args = append(args, "--rate", p.Rate)
			}
			return args
		},
	})

	return p
}
