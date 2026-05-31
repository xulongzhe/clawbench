//nolint:govet // shadowed err is acceptable in sequential blocks
package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ---------- Help definitions ----------

var taskSubcommands = []CmdHelp{
	{Name: "create", Desc: "Create a new scheduled task"},
	{Name: "list", Desc: "List all tasks"},
	{Name: "get", Desc: "Get task details by ID"},
	{Name: "list-exec", Desc: "List recent task executions"},
	{Name: "update", Desc: "Update an existing task"},
	{Name: "delete", Desc: "Delete a task"},
	{Name: "delete-exec", Desc: "Delete a task execution record"},
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
  {"ok":true,"task":{"id":1,"name":"...","status":"active",...}}
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
  {"ok":true,"tasks":[{"id":1,"name":"...","status":"active","cron_expr":"0 9 * * *","agent_id":"codebuddy","repeat_mode":"unlimited","run_count":5,"max_runs":0,...}]}
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
		`clawbench task get 1 --project /path/to/project`,
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
		`clawbench task update 1 --cron "0 10 * * 1-5"`,
		`clawbench task update 1 --prompt "Updated prompt" --repeat limited --max-runs 5`,
		`clawbench task update 1 --prompt @/path/to/prompt.txt`,
	},
}

var deleteHelp = HelpInfo{
	Usage:       "clawbench task delete TASK_ID --project PATH",
	Description: "Delete a task permanently.",
	Positional:  "TASK_ID  (required) ID of the task to delete",
	Flags: []FlagHelp{
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task delete 1`,
	},
}

var deleteExecHelp = HelpInfo{
	Usage:       "clawbench task delete-exec TASK_ID EXEC_ID --project PATH",
	Description: "Delete a single task execution record. Running executions cannot be deleted.",
	Positional:  "TASK_ID  (required) ID of the task\nEXEC_ID  (required) ID of the execution to delete",
	Flags: []FlagHelp{
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task delete-exec 1 5 --project /path/to/project`,
	},
}

var listExecHelp = HelpInfo{
	Usage:       "clawbench task list-exec TASK_ID [flags]",
	Description: "List recent task executions with status and summary.",
	Positional:  "TASK_ID  (required) ID of the task",
	Flags: []FlagHelp{
		{Name: "limit", Type: "int", Default: "1", Desc: "Max number of executions to return"},
		{Name: "project", Type: "string", Desc: "Project path", Required: true},
	},
	Examples: []string{
		`clawbench task list-exec 1 --project /path/to/project`,
		`clawbench task list-exec 1 --limit 10 --project /path/to/project`,
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
		`clawbench task pause 1`,
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
		`clawbench task resume 1`,
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
		`clawbench task trigger 1`,
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
//
// Security: @path is restricted to files under the project directory to prevent
// arbitrary file reads by AI agents (ISS-026). If projectPath is empty, the
// restriction is not applied (allows non-task CLI usage).
func readFlagOrFile(val string, projectPath string) (string, error) {
	if !strings.HasPrefix(val, "@") {
		return val, nil
	}
	path := val[1:]
	if path == "" {
		return "", fmt.Errorf("empty file path after @")
	}

	// Resolve to absolute path for containment check
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path %s: %w", path, err)
	}

	// Security: only allow reading files within the project directory
	if projectPath != "" {
		absProject, err := filepath.Abs(projectPath)
		if err != nil {
			return "", fmt.Errorf("invalid project path %s: %w", projectPath, err)
		}
		// Resolve symlinks on both sides for robust containment check
		evalProject, err := filepath.EvalSymlinks(absProject)
		if err == nil {
			absProject = evalProject
		}
		// For the file path, try to resolve; if file doesn't exist, resolve parent
		evalPath, err := filepath.EvalSymlinks(absPath)
		if err == nil {
			absPath = evalPath
		} else if os.IsNotExist(err) {
			// File doesn't exist yet — resolve the parent directory
			parent := filepath.Dir(absPath)
			evalParent, err := filepath.EvalSymlinks(parent)
			if err != nil {
				return "", fmt.Errorf("cannot resolve parent directory for %s: %w", path, err)
			}
			absPath = filepath.Join(evalParent, filepath.Base(absPath))
		} else {
			return "", fmt.Errorf("cannot resolve path %s: %w", path, err)
		}

		if !strings.HasPrefix(absPath, absProject+string(filepath.Separator)) && absPath != absProject {
			return "", fmt.Errorf("access denied: file %s is outside project directory %s", path, projectPath)
		}
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", path, err)
	}
	return string(data), nil
}

// reorderFlagsFirst reorders args so that all flag arguments (and their values)
// come before positional arguments. This works around Go's flag package behavior
// where parsing stops at the first non-flag argument.
//
// Example: ["1", "--prompt", "hello", "--project", "/path"]
//
//	→ ["--prompt", "hello", "--project", "/path", "1"]
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
	case "list-exec":
		return runListExec(args[1:])
	case "update":
		return runUpdate(args[1:])
	case "delete":
		return runDelete(args[1:])
	case "delete-exec":
		return runDeleteExec(args[1:])
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
	promptVal, err := readFlagOrFile(*prompt, *projectPath)
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
	if err := checkHTTPResponse(result, status, "create task"); err != nil {
		return outputError(err.Error())
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
	if err := checkHTTPResponse(result, status, "list tasks"); err != nil {
		return outputError(err.Error())
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
	if err := checkHTTPResponse(result, status, "get task"); err != nil {
		return outputError(err.Error())
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runListExec(args []string) int {
	args = reorderFlagsFirst(args)
	fs := flagSet("list-exec")
	limit := fs.Int("limit", 1, "Max number of executions to return")
	projectPath := fs.String("project", "", "Project path")
	parseOrHelp(fs, args, &listExecHelp)

	remaining := fs.Args()
	if len(remaining) == 0 {
		return outputError("task ID required")
	}
	if *projectPath == "" {
		return outputError("missing required flag: --project")
	}
	if *limit <= 0 {
		return outputError("--limit must be a positive integer")
	}
	taskID := remaining[0]

	path := fmt.Sprintf("/api/tasks/%s/executions?limit=%d", taskID, *limit)
	result, status, err := httpDoWithProject(http.MethodGet, path, nil, *projectPath)
	if err != nil {
		return outputError(fmt.Sprintf("failed to list executions: %v", err))
	}
	if err := checkHTTPResponse(result, status, "list executions"); err != nil {
		return outputError(err.Error())
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runUpdate(args []string) int { //nolint:gocyclo // multi-flag task update CLI
	// Anti-recursion: scheduled executions cannot modify tasks either.
	// While 'create' creates new tasks, 'update' can modify existing task
	// prompts/crons to achieve recursive behavior (ISS-031).
	if os.Getenv("CLAWBENCH_SCHEDULED") == "1" {
		return outputError("scheduled execution cannot modify tasks")
	}

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
		val, err := readFlagOrFile(*prompt, *projectPath)
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
	if err := checkHTTPResponse(result, status, "update task"); err != nil {
		return outputError(err.Error())
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
	if err := checkHTTPResponse(result, status, "delete task"); err != nil {
		return outputError(err.Error())
	}

	fmt.Println(mustMarshal(result))
	return 0
}

func runDeleteExec(args []string) int {
	args = reorderFlagsFirst(args)
	fs := flagSet("delete-exec")
	projectPath := fs.String("project", "", "Project path")
	parseOrHelp(fs, args, &deleteExecHelp)

	remaining := fs.Args()
	if len(remaining) < 2 {
		return outputError("task ID and execution ID required")
	}
	if *projectPath == "" {
		return outputError("missing required flag: --project")
	}
	taskID := remaining[0]
	execID := remaining[1]

	body := map[string]any{
		"action":      "deleteExecution",
		"executionId": execID,
	}
	result, status, err := httpDoWithProject(http.MethodPut, "/api/tasks/"+taskID, body, *projectPath)
	if err != nil {
		return outputError(fmt.Sprintf("failed to delete execution: %v", err))
	}
	if err := checkHTTPResponse(result, status, "delete execution"); err != nil {
		return outputError(err.Error())
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
	if err := checkHTTPResponse(result, status, "pause task"); err != nil {
		return outputError(err.Error())
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
	if err := checkHTTPResponse(result, status, "resume task"); err != nil {
		return outputError(err.Error())
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
	if err := checkHTTPResponse(result, status, "trigger task"); err != nil {
		return outputError(err.Error())
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
	if err := checkHTTPResponse(result, status, "list agents"); err != nil {
		return outputError(err.Error())
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
