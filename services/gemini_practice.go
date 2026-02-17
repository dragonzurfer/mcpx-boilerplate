package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type PracticeAnalysisInput struct {
	Prompt string
}

func (c *GeminiClient) GeneratePracticeAnalysis(ctx context.Context, input PracticeAnalysisInput) (AnalysisResult, error) {
	if err := c.ensureReady(); err != nil {
		return AnalysisResult{}, err
	}

	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return AnalysisResult{}, errors.New("prompt missing")
	}

	payload := buildGeminiTextPayload(prompt)
	rawResponse, err := c.sendGenerateContent(ctx, payload)
	if err != nil {
		return AnalysisResult{}, err
	}

	return parsePracticeAnalysis(rawResponse)
}

func parsePracticeAnalysis(raw []byte) (AnalysisResult, error) {
	text, err := parseGeminiText(raw)
	if err != nil {
		return AnalysisResult{}, err
	}

	output := AnalysisResult{}
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return AnalysisResult{}, err
	}

	if strings.TrimSpace(output.Summary) == "" {
		return AnalysisResult{}, errors.New("analysis empty")
	}

	return output, nil
}
