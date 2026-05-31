package model

import "time"

// ScheduledTask represents a cron-scheduled AI task.
type ScheduledTask struct {
	ID                int64                  `json:"id"`
	ProjectPath       string                 `json:"projectPath"`
	Name              string                 `json:"name"`
	Description       string                 `json:"description,omitempty"`
	CronExpr          string                 `json:"cronExpr"`
	AgentID           string                 `json:"agentId"`
	Prompt            string                 `json:"prompt"`
	SessionID         string                 `json:"sessionId,omitempty"`
	Status            string                 `json:"status"`     // active / paused / completed
	RepeatMode        string                 `json:"repeatMode"` // once / limited / unlimited
	MaxRuns           int                    `json:"maxRuns"`
	LastRunAt         *time.Time             `json:"lastRunAt,omitempty"`
	NextRunAt         *time.Time             `json:"nextRunAt,omitempty"`
	RunCount          int                    `json:"runCount"`
	LastReadAt        *time.Time             `json:"lastReadAt,omitempty"`
	UnreadCount       int                    `json:"unreadCount,omitempty"`
	CreatedAt         time.Time              `json:"createdAt"`
	UpdatedAt         time.Time              `json:"updatedAt"`
	RunningExecutions []RunningExecutionView `json:"runningExecutions,omitempty"`
	RunningCount      int                    `json:"runningCount,omitempty"`
}

// RunningExecutionView is the frontend-facing representation of a running task execution.
type RunningExecutionView struct {
	ID          string    `json:"id"`
	StartedAt   time.Time `json:"startedAt"`
	TriggerType string    `json:"triggerType"` // "auto" | "manual"
}
