package stores

import (
	"encoding/json"
	"strings"
)

type CourseMetadata struct {
	TargetAudience    []string `json:"target_audience,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	Difficulty        string   `json:"difficulty,omitempty"`
	ModuleCount       int      `json:"module_count,omitempty"`
	LessonCount       int      `json:"lesson_count,omitempty"`
	EstimatedDuration string   `json:"estimated_duration,omitempty"`
	SkillsCovered     []string `json:"skills_covered,omitempty"`
	Highlights        []string `json:"highlights,omitempty"`
}

func ParseCourseMetadata(rawJSON string) CourseMetadata {
	trimmedJSON := strings.TrimSpace(rawJSON)
	if trimmedJSON == "" {
		return CourseMetadata{}
	}

	courseMetadata := CourseMetadata{}
	if err := json.Unmarshal([]byte(trimmedJSON), &courseMetadata); err != nil {
		return CourseMetadata{}
	}

	return normalizeCourseMetadata(courseMetadata)
}

func SerializeCourseMetadata(courseMetadata CourseMetadata) string {
	normalizedMetadata := normalizeCourseMetadata(courseMetadata)

	encodedJSON, err := json.Marshal(normalizedMetadata)
	if err != nil {
		return "{}"
	}

	return string(encodedJSON)
}

func normalizeCourseMetadata(courseMetadata CourseMetadata) CourseMetadata {
	normalizedMetadata := CourseMetadata{
		TargetAudience:    cleanStringList(courseMetadata.TargetAudience),
		Tags:              cleanStringList(courseMetadata.Tags),
		Difficulty:        strings.TrimSpace(courseMetadata.Difficulty),
		ModuleCount:       courseMetadata.ModuleCount,
		LessonCount:       courseMetadata.LessonCount,
		EstimatedDuration: strings.TrimSpace(courseMetadata.EstimatedDuration),
		SkillsCovered:     cleanStringList(courseMetadata.SkillsCovered),
		Highlights:        cleanStringList(courseMetadata.Highlights),
	}

	if normalizedMetadata.ModuleCount < 0 {
		normalizedMetadata.ModuleCount = 0
	}
	if normalizedMetadata.LessonCount < 0 {
		normalizedMetadata.LessonCount = 0
	}

	return normalizedMetadata
}

func cleanStringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	cleanedValues := make([]string, 0, len(values))
	seenValues := map[string]bool{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		if seenValues[trimmedValue] {
			continue
		}
		seenValues[trimmedValue] = true
		cleanedValues = append(cleanedValues, trimmedValue)
	}

	return cleanedValues
}
