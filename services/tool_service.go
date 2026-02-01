package services

import (
	"strings"
	"time"

	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type ToolService struct {
	Store *stores.Store
}

type ToolActionInput struct {
	Tool      stores.ToolModel
	UserID    uint
	Action    string
	StageKey  string
	UsedAudio bool
	Metadata  map[string]interface{}
	Now       time.Time
}

type ToolActionOutput struct {
	Allowed           bool
	Reason            string
	UsageState        stores.ToolUsageState
	EntitlementActive bool
	FreeRules         stores.ToolFreeRules
}

func (s *ToolService) HandleAction(input ToolActionInput) (ToolActionOutput, error) {
	if input.Tool.ID == 0 || input.UserID == 0 {
		return ToolActionOutput{}, gorm.ErrInvalidData
	}
	action := strings.TrimSpace(strings.ToLower(input.Action))
	if action == "" {
		return ToolActionOutput{}, gorm.ErrInvalidData
	}

	cfg := stores.ParseToolConfig(input.Tool.ConfigJSON)
	entitlementActive := hasActiveEntitlement(s.Store, input.UserID, input.Now)

	if !input.Tool.IsActive {
		return ToolActionOutput{Allowed: false, Reason: "INACTIVE", EntitlementActive: entitlementActive, FreeRules: cfg.FreeRules}, nil
	}

	if input.Tool.IsPaidTool && !entitlementActive {
		return ToolActionOutput{Allowed: false, Reason: "PAID_REQUIRED", EntitlementActive: entitlementActive, FreeRules: cfg.FreeRules}, nil
	}

	var output ToolActionOutput
	err := s.Store.DB().Transaction(func(tx *gorm.DB) error {
		usage, state, err := s.Store.GetOrCreateToolUsage(stores.ToolUsageLookupInput{
			UserID:    input.UserID,
			ToolID:    input.Tool.ID,
			Now:       input.Now,
			ForUpdate: true,
			Tx:        tx,
		})
		if err != nil {
			return err
		}

		allowed, reason := applyToolAction(toolActionStateInput{
			Action:            action,
			StageKey:          input.StageKey,
			UsedAudio:         input.UsedAudio,
			EntitlementActive: entitlementActive,
			Config:            cfg,
			State:             &state,
		})

		if allowed {
			usage.LastUsedAt = input.Now
			if usage.FirstUsedAt.IsZero() {
				usage.FirstUsedAt = input.Now
			}
			if action == "session_start" {
				usage.TotalSessions++
			}
			if err := s.Store.SaveToolUsage(stores.ToolUsageSaveInput{Tx: tx, Usage: usage, State: state, Now: input.Now}); err != nil {
				return err
			}
		}

		metadata := map[string]interface{}{
			"allowed":    allowed,
			"reason":     reason,
			"stage_key":  input.StageKey,
			"used_audio": input.UsedAudio,
		}
		for key, value := range input.Metadata {
			metadata[key] = value
		}

		_ = s.Store.CreateToolEvent(stores.ToolEventInput{
			Tx:        tx,
			ToolID:    input.Tool.ID,
			UserID:    input.UserID,
			EventName: action,
			Metadata:  metadata,
			CreatedAt: input.Now,
		})

		output = ToolActionOutput{
			Allowed:           allowed,
			Reason:            reason,
			UsageState:        state,
			EntitlementActive: entitlementActive,
			FreeRules:         cfg.FreeRules,
		}
		return nil
	})

	if err != nil {
		return ToolActionOutput{}, err
	}
	return output, nil
}

type toolActionStateInput struct {
	Action            string
	StageKey          string
	UsedAudio         bool
	EntitlementActive bool
	Config            stores.ToolConfig
	State             *stores.ToolUsageState
}

func applyToolAction(input toolActionStateInput) (bool, string) {
	if input.State == nil {
		return false, "STATE_MISSING"
	}

	switch input.Action {
	case "session_start":
		return true, ""
	case "resume_upload":
		if input.EntitlementActive {
			return true, ""
		}
		if input.Config.FreeRules.SingleUseFreeFlow && input.State.HasUsedFreeFlow {
			return false, "FREE_FLOW_USED"
		}
		return true, ""
	case "flow_complete":
		if !input.EntitlementActive {
			input.State.HasUsedFreeFlow = true
		}
		return true, ""
	case "stage_complete":
		if input.StageKey != "" {
			input.State.CompletedStages = appendUnique(input.State.CompletedStages, input.StageKey)
		}
		return true, ""
	case "response":
		return applyResponseAction(input)
	default:
		return false, "UNKNOWN_ACTION"
	}
}

func applyResponseAction(input toolActionStateInput) (bool, string) {
	if input.EntitlementActive {
		return true, ""
	}

	maxResponses := input.Config.FreeRules.MaxFreeResponses
	if maxResponses <= 0 {
		maxResponses = 0
	}
	if input.State.FreeUserResponsesUsed >= maxResponses {
		return false, "FREE_RESPONSE_LIMIT"
	}

	if input.UsedAudio {
		maxAudio := input.Config.FreeRules.MaxFreeAudioInputs
		if maxAudio <= 0 {
			return false, "FREE_AUDIO_LIMIT"
		}
		if input.State.FreeAudioInputsUsed >= maxAudio {
			return false, "FREE_AUDIO_LIMIT"
		}
		input.State.FreeAudioInputsUsed++
	}

	input.State.FreeUserResponsesUsed++
	if input.Config.FreeRules.SingleUseFreeFlow && input.State.FreeUserResponsesUsed >= maxResponses {
		input.State.HasUsedFreeFlow = true
	}
	return true, ""
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if strings.EqualFold(item, value) {
			return items
		}
	}
	return append(items, value)
}
