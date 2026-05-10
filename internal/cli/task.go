package cli

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

// ---------- Help definitions ----------

var taskSubcommands = []CmdHelp{
	{Name: "create", Desc: "Create a new scheduled task"},
	{Name: "list", Desc: "List all tasks"},
	{Name: "get", Desc: "Get task details by ID"},
	{Name: "update", Desc: "Update an existing task"},
	{Name: "delete", Desc: "Delete a task"},
	{Name: "pause", Desc: "Pause a task's cron schedule"},
	{Name: "resume", Desc: "Resume a paused task"},
	{Name: "trigger", Desc: "Run a task immediately"},
	{Name: "list-agents", Desc: "List available agent IDs and descriptions"},
}

var createHelp = HelpInfo{
	Usage:       "clawbench task create [flags]",
	Description: "Create a new scheduled task.",
	Flags: []FlagHelp{
		{Name: "name", Type: "string", Desc: "Brief task name", Required: true},
		{Name: "cron", Type: "string", Desc: "5-field cron expression (min hour day month weekday)", Required: true},
		{Name: "agent", Type: "string", Desc: "Agent ID (run 'clawbench task list-agents' to see available)", Required: true},
		{Name: "prompt", Type: "string", Desc: "Full prompt text, or @path to read from file", Required: true},
		{Name: "repeat", Type: "string", Default: "unlimited", Desc: "Repeat mode: once|limited|unlimited"},
		{Name: "max-runs", Type: "int", Default: "0", Desc: "Max runs (required when --repeat=limited)"},
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task create --name "Daily Review" --cron "0 9 * * *" --agent codebuddy --prompt "Review recent changes" --repeat unlimited`,
		`clawbench task create --name "One-off cleanup" --cron "30 8 1 6 *" --agent claude --prompt @/path/to/prompt.txt --repeat once`,
	},
	Footer: `Cron Expression Quick Reference:
  0 9 * * *       Every day at 9:00
  */30 * * * *    Every 30 minutes
  0 9 * * 1-5     Weekdays at 9:00
  0 0 1 * *       First day of each month
  30 8 * * 1      Every Monday at 8:30

Response format:
  {"ok":true,"task":{"id":"task-xxx","name":"...","status":"active",...}}
  {"ok":false,"error":"..."}`,
}

var listHelp = HelpInfo{
	Usage:       "clawbench task list --project PATH",
	Description: "List all scheduled tasks for the project.",
	Flags: []FlagHelp{
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task list --project /path/to/project`,
	},
	Footer: `Response format:
  {"ok":true,"tasks":[{"id":"task-xxx","name":"...","status":"active","cron_expr":"0 9 * * *","agent_id":"codebuddy","repeat_mode":"unlimited","run_count":5,"max_runs":0,...}]}
  {"ok":false,"error":"..."}`,
}

var getHelp = HelpInfo{
	Usage:       "clawbench task get TASK_ID --project PATH",
	Description: "Get detailed information about a specific task by ID, including running executions.",
	Positional:  "TASK_ID  (required) ID of the task to retrieve",
	Flags: []FlagHelp{
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task get task-abc123 --project /path/to/project`,
	},
}

var updateHelp = HelpInfo{
	Usage:       "clawbench task update TASK_ID [flags]",
	Description: "Update an existing task. Only provide fields you want to change. Updating a completed task reactivates it.",
	Positional:  "TASK_ID  (required) ID of the task to update",
	Flags: []FlagHelp{
		{Name: "name", Type: "string", Desc: "Task name"},
		{Name: "cron", Type: "string", Desc: "Cron expression"},
		{Name: "agent", Type: "string", Desc: "Agent ID"},
		{Name: "prompt", Type: "string", Desc: "Prompt text, or @path to read from file"},
		{Name: "repeat", Type: "string", Desc: "Repeat mode: once|limited|unlimited"},
		{Name: "max-runs", Type: "int", Default: "-1", Desc: "Max runs"},
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task update task-abc123 --cron "0 10 * * 1-5"`,
		`clawbench task update task-abc123 --prompt "Updated prompt" --repeat limited --max-runs 5`,
		`clawbench task update task-abc123 --prompt @/path/to/prompt.txt`,
	},
}

var deleteHelp = HelpInfo{
	Usage:       "clawbench task delete TASK_ID --project PATH",
	Description: "Delete a task. Soft-deletes — the task will no longer appear in lists.",
	Positional:  "TASK_ID  (required) ID of the task to delete",
	Flags: []FlagHelp{
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task delete task-abc123`,
	},
}

var pauseHelp = HelpInfo{
	Usage:       "clawbench task pause TASK_ID --project PATH",
	Description: "Pause a task's cron schedule. The task will not execute until resumed.",
	Positional:  "TASK_ID  (required) ID of the task to pause",
	Flags: []FlagHelp{
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task pause task-abc123`,
	},
}

var resumeHelp = HelpInfo{
	Usage:       "clawbench task resume TASK_ID --project PATH",
	Description: "Resume a paused task. The cron schedule is reactivated.",
	Positional:  "TASK_ID  (required) ID of the task to resume",
	Flags: []FlagHelp{
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task resume task-abc123`,
	},
}

var triggerHelp = HelpInfo{
	Usage:       "clawbench task trigger TASK_ID --project PATH",
	Description: "Run a task immediately, regardless of the cron schedule. Does not affect the schedule.",
	Positional:  "TASK_ID  (required) ID of the task to trigger",
	Flags: []FlagHelp{
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task trigger task-abc123`,
	},
}

var listAgentsHelp = HelpInfo{
	Usage:       "clawbench task list-agents",
	Description: "List available agent IDs and their descriptions. Use an agent ID with 'clawbench task create --agent'.",
	Examples: []string{
		`clawbench task list-agents`,
	},
}

// readFlagOrFile returns the value as-is, unless it starts with "@" in which
// case the rest is treated as a file path and the file's contents are returned.
// This allows passing long text (e.g. --prompt @/path/to/file.txt) without
// shell variable expansion or argument length issues.
func readFlagOrFile(val string) (string, error) {
	if !strings.HasPrefix(val, "@") {
		return val, nil
	}
	path := val[1:]
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", path, err)
	}
	return string(data), nil
}

// reorderFlagsFirst reorders args so that all flag arguments (and their values)
// come before positional arguments. This works around Go's flag package behavior
// where parsing stops at the first non-flag argument.
//
// Example: ["task-abc", "--prompt", "hello", "--project", "/path"]
//       → ["--prompt", "hello", "--project", "/path", "task-abc"]
func reorderFlagsFirst(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			// If this flag takes a value (doesn't end with = and next arg is not a flag),
			// include the value as part of the flag group.
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}

// ---------- Command dispatch ----------

// RunTaskCommand dispatches "clawbench task <subcommand>" CLI invocations.
// Task operations are routed through the server's HTTP API to avoid
// concurrent database access from a separate process.
func RunTaskCommand(args []string) int {
	if len(args) == 0 {
		printGroupHelp("clawbench task <subcommand> [options]", "Manage scheduled AI tasks with cron-based execution.", taskSubcommands)
		return 0
	}

	// Handle top-level --help
	if args[0] == "--help" || args[0] == "-h" {
		printGroupHelp("clawbench task <subcommand> [options]", "Manage scheduled AI tasks with cron-based execution.", taskSubcommands)
		return 0
	}

	// Load config to determine server port and auth credentials.
	loadConfig()

	// Dispatch to subcommand
	switch args[0] {
	case "create":
		return runCreate(args[1:])
	case "list":
		return runList(args[1:])
	case "get":
		return runGet(args[1:])
	case "update":
		return runUpdate(args[1:])
	case "delete":
		return runDelete(args[1:])
	case "pause":
		return runPause(args[1:])
	case "resume":
		return runResume(args[1:])
	case "trigger":
		return runTrigger(args[1:])
	case "list-agents":
		return runListAgents(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", args[0])
		printGroupHelp("clawbench task <subcommand> [options]", "Manage scheduled AI tasks with cron-based execution.", taskSubcommands)
		return 1
	}
}

// ---------- Subcommands ----------

func runCreate(args []string) int {
	// Anti-recursion: scheduled executions cannot create new tasks
	if os.Getenv("CLAWBENCH_SCHEDULED") == "1" {
		return outputError("scheduled execution cannot create new tasks")
	}

	fs := flagSet("create")
	name := fs.String("name", "", "Task name (required)")
	cronExpr := fs.String("cron", "", "Cron expression (required)")
	agentID := fs.String("agent", "", "Agent ID (required)")
	prompt := fs.String("prompt", "", "Prompt for each execution (required)")
	repeatMode := fs.String("repeat", "unlimited", "Repeat mode: once|limited|unlimited")
	maxRuns := fs.Int("max-runs", 0, "Max runs (required when repeat=limited)")
	projectPath := fs.String("project", "", "Project path")
	parseOrHelp(fs, args, &createHelp)

	if *name == "" || *cronExpr == "" || *agentID == "" || *prompt == "" || *projectPath == "" {
		return outputError("missing required fields: --name, --cron, --agent, --prompt, --project")
	}
	if *repeatMode != "once" && *repeatMode != "limited" && *repeatMode != "unlimited" {
		return outputError("invalid --repeat: must be once|limited|unlimited")
	}
	if *repeatMode == "limited" && *maxRuns <= 0 {
		return outputError("--max-runs required when --repeat=limited")
	}

	// Resolve @file syntax for prompt
	promptVal, err := readFlagOrFile(*prompt)
	if err != nil {
		return outputError(fmt.Sprintf("%v", err))
	}

	body := map[string]any{
		"name":        *name,
		"cron_expr":   *cronExpr,
		"agent_id":    *agentID,
		"prompt":      promptVal,
		"repeat_mode": *repeatMode,
		"max_runs":    *maxRuns,
	}

	result, status, err := httpDoWithProject(http.MethodPost, "/api/tasks", body, *projectPath)
	if err != nil {
		return outputError(fmt.Sprintf("failed to create task: %v", err))
	}
	if status != http.StatusOK {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", status)
		}
		return outputError(fmt.Sprintf("failed to create task: %s", errMsg))
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runList(args []string) int {
	fs := flagSet("list")
	projectPath := fs.String("project", "", "Project path")
	parseOrHelp(fs, args, &listHelp)

	if *projectPath == "" {
		return outputError("missing required flag: --project")
	}

	result, status, err := httpDoWithProject(http.MethodGet, "/api/tasks", nil, *projectPath)
	if err != nil {
		return outputError(fmt.Sprintf("failed to list tasks: %v", err))
	}
	if status != http.StatusOK {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", status)
		}
		return outputError(fmt.Sprintf("failed to list tasks: %s", errMsg))
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runGet(args []string) int {
	args = reorderFlagsFirst(args)
	fs := flagSet("get")
	projectPath := fs.String("project", "", "Project path")
	parseOrHelp(fs, args, &getHelp)

	remaining := fs.Args()
	if len(remaining) == 0 {
		return outputError("task ID required")
	}
	if *projectPath == "" {
		return outputError("missing required flag: --project")
	}
	taskID := remaining[0]

	result, status, err := httpDoWithProject(http.MethodGet, "/api/tasks/"+taskID, nil, *projectPath)
	if err != nil {
		return outputError(fmt.Sprintf("failed to get task: %v", err))
	}
	if status != http.StatusOK {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", status)
		}
		return outputError(fmt.Sprintf("failed to get task: %s", errMsg))
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runUpdate(args []string) int {
	// Go's flag package stops parsing at the first non-flag argument.
	// "clawbench task update task-ID --prompt text" would fail because task-ID
	// comes before --prompt. Reorder so all flags come first, then positional args.
	args = reorderFlagsFirst(args)

	fs := flagSet("update")
	name := fs.String("name", "", "Task name")
	cronExpr := fs.String("cron", "", "Cron expression")
	agentID := fs.String("agent", "", "Agent ID")
	prompt := fs.String("prompt", "", "Prompt text, or @path to read from file")
	repeatMode := fs.String("repeat", "", "Repeat mode: once|limited|unlimited")
	maxRuns := fs.Int("max-runs", -1, "Max runs")
	projectPath := fs.String("project", "", "Project path")
	parseOrHelp(fs, args, &updateHelp)

	remaining := fs.Args()
	if len(remaining) == 0 {
		return outputError("task ID required")
	}
	taskID := remaining[0]

	if *projectPath == "" {
		return outputError("missing required flag: --project")
	}

	// Resolve @file syntax for prompt
	promptVal := ""
	if *prompt != "" {
		val, err := readFlagOrFile(*prompt)
		if err != nil {
			return outputError(fmt.Sprintf("%v", err))
		}
		promptVal = val
	}

	body := map[string]any{
		"action": "update",
	}
	if *name != "" {
		body["name"] = *name
	}
	if *cronExpr != "" {
		body["cron_expr"] = *cronExpr
	}
	if *agentID != "" {
		body["agent_id"] = *agentID
	}
	if promptVal != "" {
		body["prompt"] = promptVal
	}
	if *repeatMode != "" {
		if *repeatMode != "once" && *repeatMode != "limited" && *repeatMode != "unlimited" {
			return outputError("invalid --repeat: must be once|limited|unlimited")
		}
		body["repeat_mode"] = *repeatMode
	}
	if *maxRuns >= 0 {
		body["max_runs"] = *maxRuns
	}

	result, status, err := httpDoWithProject(http.MethodPut, "/api/tasks/"+taskID, body, *projectPath)
	if err != nil {
		return outputError(fmt.Sprintf("failed to update task: %v", err))
	}
	if status != http.StatusOK {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", status)
		}
		return outputError(fmt.Sprintf("failed to update task: %s", errMsg))
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runDelete(args []string) int {
	args = reorderFlagsFirst(args)
	fs := flagSet("delete")
	projectPath := fs.String("project", "", "Project path")
	parseOrHelp(fs, args, &deleteHelp)

	remaining := fs.Args()
	if len(remaining) == 0 {
		return outputError("task ID required")
	}
	if *projectPath == "" {
		return outputError("missing required flag: --project")
	}
	taskID := remaining[0]

	result, status, err := httpDoWithProject(http.MethodDelete, "/api/tasks/"+taskID, nil, *projectPath)
	if err != nil {
		return outputError(fmt.Sprintf("failed to delete task: %v", err))
	}
	if status != http.StatusOK {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", status)
		}
		return outputError(fmt.Sprintf("failed to delete task: %s", errMsg))
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runPause(args []string) int {
	args = reorderFlagsFirst(args)
	fs := flagSet("pause")
	projectPath := fs.String("project", "", "Project path")
	parseOrHelp(fs, args, &pauseHelp)

	remaining := fs.Args()
	if len(remaining) == 0 {
		return outputError("task ID required")
	}
	if *projectPath == "" {
		return outputError("missing required flag: --project")
	}
	taskID := remaining[0]

	body := map[string]any{"action": "pause"}
	result, status, err := httpDoWithProject(http.MethodPut, "/api/tasks/"+taskID, body, *projectPath)
	if err != nil {
		return outputError(fmt.Sprintf("failed to pause task: %v", err))
	}
	if status != http.StatusOK {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", status)
		}
		return outputError(fmt.Sprintf("failed to pause task: %s", errMsg))
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runResume(args []string) int {
	args = reorderFlagsFirst(args)
	fs := flagSet("resume")
	projectPath := fs.String("project", "", "Project path")
	parseOrHelp(fs, args, &resumeHelp)

	remaining := fs.Args()
	if len(remaining) == 0 {
		return outputError("task ID required")
	}
	if *projectPath == "" {
		return outputError("missing required flag: --project")
	}
	taskID := remaining[0]

	body := map[string]any{"action": "resume"}
	result, status, err := httpDoWithProject(http.MethodPut, "/api/tasks/"+taskID, body, *projectPath)
	if err != nil {
		return outputError(fmt.Sprintf("failed to resume task: %v", err))
	}
	if status != http.StatusOK {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", status)
		}
		return outputError(fmt.Sprintf("failed to resume task: %s", errMsg))
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runTrigger(args []string) int {
	args = reorderFlagsFirst(args)
	fs := flagSet("trigger")
	projectPath := fs.String("project", "", "Project path")
	parseOrHelp(fs, args, &triggerHelp)

	remaining := fs.Args()
	if len(remaining) == 0 {
		return outputError("task ID required")
	}
	if *projectPath == "" {
		return outputError("missing required flag: --project")
	}
	taskID := remaining[0]

	body := map[string]any{"action": "trigger"}
	result, status, err := httpDoWithProject(http.MethodPut, "/api/tasks/"+taskID, body, *projectPath)
	if err != nil {
		return outputError(fmt.Sprintf("failed to trigger task: %v", err))
	}
	if status != http.StatusOK {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", status)
		}
		return outputError(fmt.Sprintf("failed to trigger task: %s", errMsg))
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runListAgents(args []string) int {
	fs := flagSet("list-agents")
	parseOrHelp(fs, args, &listAgentsHelp)

	result, status, err := httpDo(http.MethodGet, "/api/agents", nil)
	if err != nil {
		return outputError(fmt.Sprintf("failed to list agents: %v", err))
	}
	if status != http.StatusOK {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", status)
		}
		return outputError(fmt.Sprintf("failed to list agents: %s", errMsg))
	}

	// Format as a readable table
	agentsRaw, ok := result["agents"].([]any)
	if !ok {
		// Fallback: just output raw JSON
		fmt.Println(mustMarshal(result))
		return 0
	}

	type agentEntry struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Specialty string `json:"specialty"`
		Backend   string `json:"backend"`
	}

	var agents []agentEntry
	for _, a := range agentsRaw {
		m, ok := a.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		name, _ := m["name"].(string)
		specialty, _ := m["specialty"].(string)
		backend, _ := m["backend"].(string)
		agents = append(agents, agentEntry{ID: id, Name: name, Specialty: specialty, Backend: backend})
	}

	outputJSON(map[string]any{
		"ok":     true,
		"agents": agents,
	})
	return 0
}
