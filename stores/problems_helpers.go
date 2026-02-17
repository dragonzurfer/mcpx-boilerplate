package stores

import (
	"encoding/json"
	"strings"
)

func ParseProblemStatement(rawJSON string) ProblemStatement {
	trimmedJSON := strings.TrimSpace(rawJSON)
	if trimmedJSON == "" {
		return ProblemStatement{}
	}

	statement := ProblemStatement{}
	if err := json.Unmarshal([]byte(trimmedJSON), &statement); err != nil {
		return ProblemStatement{}
	}

	return normalizeProblemStatement(statement)
}

func SerializeProblemStatement(statement ProblemStatement) string {
	normalized := normalizeProblemStatement(statement)

	encodedJSON, err := json.Marshal(normalized)
	if err != nil {
		return "{}"
	}

	return string(encodedJSON)
}

func normalizeProblemStatement(statement ProblemStatement) ProblemStatement {
	normalized := ProblemStatement{
		Markdown: strings.TrimSpace(statement.Markdown),
		Examples: normalizeProblemExamples(statement.Examples),
		Notes:    cleanStringList(statement.Notes),
	}

	return normalized
}

func normalizeProblemExamples(examples []ProblemExample) []ProblemExample {
	if len(examples) == 0 {
		return []ProblemExample{}
	}

	normalizedExamples := make([]ProblemExample, 0, len(examples))
	for _, example := range examples {
		normalizedExample := normalizeProblemExample(example)
		if normalizedExample.Input == "" && normalizedExample.Output == "" && normalizedExample.Explanation == "" {
			continue
		}
		normalizedExamples = append(normalizedExamples, normalizedExample)
	}

	return normalizedExamples
}

func normalizeProblemExample(example ProblemExample) ProblemExample {
	return ProblemExample{
		Input:       strings.TrimSpace(example.Input),
		Output:      strings.TrimSpace(example.Output),
		Explanation: strings.TrimSpace(example.Explanation),
	}
}

func ParseProblemEditorial(rawJSON string) ProblemEditorial {
	trimmedJSON := strings.TrimSpace(rawJSON)
	if trimmedJSON == "" {
		return ProblemEditorial{}
	}

	editorial := ProblemEditorial{}
	if err := json.Unmarshal([]byte(trimmedJSON), &editorial); err != nil {
		return ProblemEditorial{}
	}

	return normalizeProblemEditorial(editorial)
}

func SerializeProblemEditorial(editorial ProblemEditorial) string {
	normalized := normalizeProblemEditorial(editorial)

	encodedJSON, err := json.Marshal(normalized)
	if err != nil {
		return "{}"
	}

	return string(encodedJSON)
}

func normalizeProblemEditorial(editorial ProblemEditorial) ProblemEditorial {
	normalized := ProblemEditorial{
		Markdown: strings.TrimSpace(editorial.Markdown),
		Hints:    cleanStringList(editorial.Hints),
	}

	return normalized
}

func ParseProblemTags(rawJSON string) []string {
	trimmedJSON := strings.TrimSpace(rawJSON)
	if trimmedJSON == "" {
		return []string{}
	}

	tags := []string{}
	if err := json.Unmarshal([]byte(trimmedJSON), &tags); err != nil {
		return []string{}
	}

	return cleanStringList(tags)
}

func SerializeProblemTags(tags []string) string {
	cleanedTags := cleanStringList(tags)

	encodedJSON, err := json.Marshal(cleanedTags)
	if err != nil {
		return "[]"
	}

	return string(encodedJSON)
}

type ProblemIOSpec struct {
	Mode string `json:"mode,omitempty"`
}

type ProblemConstraints struct {
	TimeLimitMs         int                          `json:"time_limit_ms,omitempty"`
	MemoryLimitKb       int                          `json:"memory_limit_kb,omitempty"`
	OutputLimitKb       int                          `json:"output_limit_kb,omitempty"`
	InputConstraintsMD  string                       `json:"input_constraints_markdown,omitempty"`
	OutputConstraintsMD string                       `json:"output_constraints_markdown,omitempty"`
	Languages           map[string]ProblemLangLimits `json:"languages,omitempty"`
}

type ProblemLangLimits struct {
	TimeLimitMs   int `json:"time_limit_ms,omitempty"`
	MemoryLimitKb int `json:"memory_limit_kb,omitempty"`
	OutputLimitKb int `json:"output_limit_kb,omitempty"`
}

func ParseProblemIOSpec(rawJSON string) ProblemIOSpec {
	trimmedJSON := strings.TrimSpace(rawJSON)
	if trimmedJSON == "" {
		return ProblemIOSpec{}
	}

	spec := ProblemIOSpec{}
	if err := json.Unmarshal([]byte(trimmedJSON), &spec); err != nil {
		return ProblemIOSpec{}
	}

	spec.Mode = strings.ToUpper(strings.TrimSpace(spec.Mode))
	return spec
}

func ParseProblemConstraints(rawJSON string) ProblemConstraints {
	trimmedJSON := strings.TrimSpace(rawJSON)
	if trimmedJSON == "" {
		return ProblemConstraints{}
	}

	constraints := ProblemConstraints{}
	if err := json.Unmarshal([]byte(trimmedJSON), &constraints); err != nil {
		return ProblemConstraints{}
	}

	return normalizeProblemConstraints(constraints)
}

func normalizeProblemConstraints(constraints ProblemConstraints) ProblemConstraints {
	normalized := ProblemConstraints{
		TimeLimitMs:         constraints.TimeLimitMs,
		MemoryLimitKb:       constraints.MemoryLimitKb,
		OutputLimitKb:       constraints.OutputLimitKb,
		InputConstraintsMD:  strings.TrimSpace(constraints.InputConstraintsMD),
		OutputConstraintsMD: strings.TrimSpace(constraints.OutputConstraintsMD),
		Languages:           normalizeProblemLanguageLimits(constraints.Languages),
	}

	if normalized.TimeLimitMs < 0 {
		normalized.TimeLimitMs = 0
	}
	if normalized.MemoryLimitKb < 0 {
		normalized.MemoryLimitKb = 0
	}
	if normalized.OutputLimitKb < 0 {
		normalized.OutputLimitKb = 0
	}

	return normalized
}

func normalizeProblemLanguageLimits(limits map[string]ProblemLangLimits) map[string]ProblemLangLimits {
	if limits == nil {
		return map[string]ProblemLangLimits{}
	}

	cleaned := map[string]ProblemLangLimits{}
	for key, value := range limits {
		languageKey := strings.ToLower(strings.TrimSpace(key))
		if languageKey == "" {
			continue
		}
		cleaned[languageKey] = normalizeProblemLangLimit(value)
	}

	return cleaned
}

func normalizeProblemLangLimit(limit ProblemLangLimits) ProblemLangLimits {
	normalized := ProblemLangLimits{
		TimeLimitMs:   limit.TimeLimitMs,
		MemoryLimitKb: limit.MemoryLimitKb,
		OutputLimitKb: limit.OutputLimitKb,
	}

	if normalized.TimeLimitMs < 0 {
		normalized.TimeLimitMs = 0
	}
	if normalized.MemoryLimitKb < 0 {
		normalized.MemoryLimitKb = 0
	}
	if normalized.OutputLimitKb < 0 {
		normalized.OutputLimitKb = 0
	}

	return normalized
}

func normalizeProblemDifficulty(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case ProblemDifficultyEasy, ProblemDifficultyMedium, ProblemDifficultyHard:
		return normalized
	default:
		return ProblemDifficultyEasy
	}
}

func normalizeProblemStatus(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case ProblemStatusDraft, ProblemStatusPublished, ProblemStatusArchived:
		return normalized
	default:
		return ProblemStatusDraft
	}
}
