package cli

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
)

// ---------- Help definitions ----------

var ragSubcommands = []CmdHelp{
	{Name: "search", Desc: "Search conversation history by semantic query"},
	{Name: "message", Desc: "Get full message detail by ID"},
	{Name: "session", Desc: "Get all messages in a session"},
}

var searchHelp = HelpInfo{
	Usage:       "clawbench rag search [flags]",
	Description: "Search conversation history by semantic query.",
	Flags: []FlagHelp{
		{Name: "q", Short: "q", Type: "string", Desc: "Search query text", Required: true},
		{Name: "limit", Type: "int", Default: "config default (5)", Desc: "Number of results"},
		{Name: "project", Type: "string", Desc: "Project path (omit for global search across all projects)"},
		{Name: "backend", Type: "string", Desc: "Filter by backend name"},
		{Name: "role", Type: "string", Desc: "Filter by role: user|assistant"},
		{Name: "session-id", Type: "string", Desc: "Limit results to this session"},
		{Name: "exclude-session-id", Type: "string", Desc: "Exclude this session from results"},
		{Name: "from", Type: "string", Desc: "Time range start"},
		{Name: "to", Type: "string", Desc: "Time range end"},
	},
	Examples: []string{
		`clawbench rag search -q "authentication bug" --limit 3 --project /path/to/project`,
		`clawbench rag search -q "RAG design"                           # global search`,
	},
	Footer: `Response format:
  {"results":[{"chunk_text":"...","score":0.85,"session_id":"...","message_id":42,...}],"total":3}

Tips:
  - chunk_text is a text excerpt; thinking blocks and tool calls are excluded from the index
  - Use "clawbench rag message <message_id>" for full message content including tool_use and thinking
  - Use "clawbench rag session <session_id>" for complete conversation flow`,
}

var messageHelp = HelpInfo{
	Usage:       "clawbench rag message [MESSAGE_ID] [--id ID] [--project PATH]",
	Description: "Get full message detail by ID, including all content blocks (text, thinking, tool_use, warning, error).",
	Flags: []FlagHelp{
		{Name: "id", Type: "string", Desc: "Message database ID (or pass as positional arg)"},
		{Name: "project", Type: "string", Desc: "Project path (omit for cross-project access)"},
	},
	Positional: "MESSAGE_ID  (optional) Message database ID",
	Examples: []string{
		`clawbench rag message --id 42 --project /path/to/project`,
		`clawbench rag message 42                     # cross-project access`,
	},
}

var sessionHelp = HelpInfo{
	Usage:       "clawbench rag session [SESSION_ID] [--id ID] [--project PATH]",
	Description: "Get all messages in a session — the complete conversation including user messages, AI responses with thinking and tool_use blocks.",
	Flags: []FlagHelp{
		{Name: "id", Type: "string", Desc: "Session ID (or pass as positional arg)"},
		{Name: "project", Type: "string", Desc: "Project path (omit for cross-project access)"},
	},
	Positional: "SESSION_ID  (optional) Session ID",
	Examples: []string{
		`clawbench rag session --id abc-123-def --project /path/to/project`,
		`clawbench rag session abc-123-def                     # cross-project access`,
	},
	Footer: `Response format:
  {"session_id":"...","messages":[...],"total":15}`,
}

// ---------- Command dispatch ----------

// RunRAGCommand dispatches "clawbench rag <subcommand>" CLI invocations.
// RAG operations are routed through the server's HTTP API to avoid
// concurrent database access from a separate process.
func RunRAGCommand(args []string) int {
	if len(args) == 0 {
		printGroupHelp("clawbench rag <subcommand> [options]", "Search and retrieve conversation history via RAG.", ragSubcommands)
		return 0
	}

	// Handle top-level --help
	if args[0] == "--help" || args[0] == "-h" {
		printGroupHelp("clawbench rag <subcommand> [options]", "Search and retrieve conversation history via RAG.", ragSubcommands)
		return 0
	}

	// Load config to determine server port and auth credentials.
	loadConfig()

	// Dispatch to subcommand
	switch args[0] {
	case "search":
		return runRAGSearch(args[1:])
	case "message":
		return runRAGMessage(args[1:])
	case "session":
		return runRAGSession(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", args[0])
		printGroupHelp("clawbench rag <subcommand> [options]", "Search and retrieve conversation history via RAG.", ragSubcommands)
		return 1
	}
}

// ---------- Subcommands ----------

func runRAGSearch(args []string) int {
	fs := flagSet("search")
	query := fs.String("q", "", "Search query (required)")
	limit := fs.Int("limit", 0, "Number of results (default from config)")
	project := fs.String("project", "", "Project path (omit for global search)")
	backend := fs.String("backend", "", "Filter by backend name")
	role := fs.String("role", "", "Filter by role: user or assistant")
	sessionID := fs.String("session-id", "", "Limit results to this session")
	excludeSessionID := fs.String("exclude-session-id", "", "Exclude this session from results")
	fromTime := fs.String("from", "", "Time range start")
	toTime := fs.String("to", "", "Time range end")
	parseOrHelp(fs, args, &searchHelp)

	if *query == "" {
		return outputError("missing required flag: -q (search query)")
	}

	body := map[string]any{
		"q":                  *query,
		"backend":            *backend,
		"role":               *role,
		"session_id":         *sessionID,
		"exclude_session_id": *excludeSessionID,
		"from":               *fromTime,
		"to":                 *toTime,
	}
	if *limit > 0 {
		body["limit"] = *limit
	}

	var result map[string]any
	var status int
	var err error
	if *project != "" {
		result, status, err = httpDoWithProject(http.MethodPost, "/api/rag/search", body, *project)
	} else {
		result, status, err = httpDo(http.MethodPost, "/api/rag/search", body)
	}
	if err != nil {
		return outputError(fmt.Sprintf("search failed: %v", err))
	}
	if err := checkHTTPResponse(result, status, "search"); err != nil {
		return outputError(err.Error())
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runRAGMessage(args []string) int {
	fs := flagSet("message")
	idStr := fs.String("id", "", "Message database ID (required)")
	project := fs.String("project", "", "Project path (omit for cross-project access)")
	parseOrHelp(fs, args, &messageHelp)

	if *idStr == "" {
		// Also accept positional arg
		if fs.NArg() > 0 {
			v := fs.Args()[0]
			idStr = &v
		} else {
			return outputError("missing required flag: --id (message ID)")
		}
	}

	id, err := strconv.ParseInt(*idStr, 10, 64)
	if err != nil {
		return outputError(fmt.Sprintf("invalid message ID: %v", err))
	}

	var result map[string]any
	var status int
	if *project != "" {
		result, status, err = httpDoWithProject(http.MethodGet, "/api/rag/message?id="+strconv.FormatInt(id, 10), nil, *project)
	} else {
		result, status, err = httpDo(http.MethodGet, "/api/rag/message?id="+strconv.FormatInt(id, 10), nil)
	}
	if err != nil {
		return outputError(fmt.Sprintf("message not found: %v", err))
	}
	if err := checkHTTPResponse(result, status, "get message"); err != nil {
		return outputError(err.Error())
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runRAGSession(args []string) int {
	fs := flagSet("session")
	sessionID := fs.String("id", "", "Session ID (required)")
	project := fs.String("project", "", "Project path (omit for cross-project access)")
	parseOrHelp(fs, args, &sessionHelp)

	if *sessionID == "" {
		if fs.NArg() > 0 {
			v := fs.Args()[0]
			sessionID = &v
		} else {
			return outputError("missing required flag: --id (session ID)")
		}
	}

	var result map[string]any
	var status int
	var err error
	if *project != "" {
		result, status, err = httpDoWithProject(http.MethodGet, "/api/rag/session?id="+*sessionID, nil, *project)
	} else {
		result, status, err = httpDo(http.MethodGet, "/api/rag/session?id="+*sessionID, nil)
	}
	if err != nil {
		return outputError(fmt.Sprintf("session not found: %v", err))
	}
	if err := checkHTTPResponse(result, status, "get session"); err != nil {
		return outputError(err.Error())
	}

	fmt.Println(mustMarshal(result))
	return 0
}
