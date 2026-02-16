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
