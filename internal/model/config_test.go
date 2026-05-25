package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSummarizeConfig_IsChatSummaryEnabled_Nil(t *testing.T) {
	cfg := SummarizeConfig{}
	// ChatSummary is nil — should default to true
	assert.True(t, cfg.IsChatSummaryEnabled())
}

func TestSummarizeConfig_IsChatSummaryEnabled_True(t *testing.T) {
	val := true
	cfg := SummarizeConfig{ChatSummary: &val}
	assert.True(t, cfg.IsChatSummaryEnabled())
}

func TestSummarizeConfig_IsChatSummaryEnabled_False(t *testing.T) {
	val := false
	cfg := SummarizeConfig{ChatSummary: &val}
	assert.False(t, cfg.IsChatSummaryEnabled())
}
