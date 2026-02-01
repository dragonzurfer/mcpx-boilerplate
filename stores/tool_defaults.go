package stores

func DefaultTools() []ToolSeedInput {
	return []ToolSeedInput{
		{
			Name:       "Career Copilot",
			Slug:       "career-copilot",
			Category:   "career",
			IsActive:   true,
			IsPaidTool: false,
			Config: ToolConfig{
				FreeRules: ToolFreeRules{
					MaxFreeResponses:   2,
					MaxFreeAudioInputs: 1,
					SingleUseFreeFlow:  true,
					RequireSignIn:      true,
				},
				Tracking: ToolTrackingRules{
					TrackUploads:   true,
					TrackResponses: true,
					TrackAudio:     true,
					TrackStages:    true,
				},
				Stages: []ToolStageDefinition{
					{Key: "upload", Label: "Resume upload"},
					{Key: "analysis", Label: "Resume analysis"},
					{Key: "focus", Label: "Focus selection"},
					{Key: "subfocus", Label: "Subfocus selection"},
					{Key: "questions", Label: "Context questions"},
					{Key: "mentor_response", Label: "Initial mentor response"},
					{Key: "chat", Label: "Chat"},
				},
			},
		},
	}
}
