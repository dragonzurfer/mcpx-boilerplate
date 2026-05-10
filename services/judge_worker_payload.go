package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

const JudgeWorkerPayloadEnv = "JUDGE_RUNNER_PAYLOAD_B64"

type JudgeWorkerPayload struct {
	RunnerInput RunnerInput `json:"runner_input"`
}

type JudgeWorkerResult struct {
	RunnerOutput *RunnerOutput `json:"runner_output,omitempty"`
	Error        string        `json:"error,omitempty"`
}

func EncodeJudgeWorkerPayload(input RunnerInput) (string, error) {
	payload := JudgeWorkerPayload{RunnerInput: input}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(rawPayload), nil
}

func DecodeJudgeWorkerPayload(encodedPayload string) (RunnerInput, error) {
	trimmedPayload := strings.TrimSpace(encodedPayload)
	if trimmedPayload == "" {
		return RunnerInput{}, errors.New("judge worker payload missing")
	}

	rawPayload, err := base64.StdEncoding.DecodeString(trimmedPayload)
	if err != nil {
		return RunnerInput{}, err
	}

	payload := JudgeWorkerPayload{}
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return RunnerInput{}, err
	}

	return payload.RunnerInput, nil
}
