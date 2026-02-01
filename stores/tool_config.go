package stores

import (
	"encoding/json"
	"strings"
)

type ToolConfig struct {
	FreeRules ToolFreeRules         `json:"free_rules"`
	Tracking  ToolTrackingRules     `json:"tracking"`
	Stages    []ToolStageDefinition `json:"stages"`
}

type ToolFreeRules struct {
	MaxFreeResponses   int  `json:"max_free_responses"`
	MaxFreeAudioInputs int  `json:"max_free_audio_inputs"`
	SingleUseFreeFlow  bool `json:"single_use_free_flow"`
	RequireSignIn      bool `json:"require_sign_in"`
}

type ToolTrackingRules struct {
	TrackUploads   bool `json:"track_uploads"`
	TrackResponses bool `json:"track_responses"`
	TrackAudio     bool `json:"track_audio"`
	TrackStages    bool `json:"track_stages"`
}

type ToolStageDefinition struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type ToolUsageState struct {
	HasUsedFreeFlow       bool     `json:"has_used_free_flow"`
	FreeUserResponsesUsed int      `json:"free_user_responses_used"`
	FreeAudioInputsUsed   int      `json:"free_audio_inputs_used"`
	CompletedStages       []string `json:"completed_stages"`
}

func ParseToolConfig(raw string) ToolConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ToolConfig{}
	}
	cfg := ToolConfig{}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return ToolConfig{}
	}
	return cfg
}

func SerializeToolConfig(cfg ToolConfig) string {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func DefaultToolUsageState() ToolUsageState {
	return ToolUsageState{
		HasUsedFreeFlow:       false,
		FreeUserResponsesUsed: 0,
		FreeAudioInputsUsed:   0,
		CompletedStages:       []string{},
	}
}

func ParseToolUsageState(raw string) ToolUsageState {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultToolUsageState()
	}
	state := ToolUsageState{}
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return DefaultToolUsageState()
	}
	if state.CompletedStages == nil {
		state.CompletedStages = []string{}
	}
	return state
}

func SerializeToolUsageState(state ToolUsageState) string {
	raw, err := json.Marshal(state)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
