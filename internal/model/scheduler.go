package model

import "time"

// ScheduledTask represents a cron-scheduled AI task.
type ScheduledTask struct {
	ID          string     `json:"id"`
	ProjectPath string     `json:"projectPath"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	CronExpr    string     `json:"cronExpr"`
	AgentID     string     `json:"agentId"`
	Prompt      string     `json:"prompt"`
	SessionID   string     `json:"sessionId,omitempty"`
	Status      string     `json:"status"`       // active / paused / completed / deleted
	RepeatMode  string     `json:"repeatMode"`   // once / limited / unlimited
	MaxRuns     int        `json:"maxRuns"`
	LastRunAt   *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt   *time.Time `json:"nextRunAt,omitempty"`
	RunCount    int        `json:"runCount"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}
