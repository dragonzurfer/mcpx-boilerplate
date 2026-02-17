package stores

import (
	"encoding/json"
	"strings"
)

type ExecutionPolicy struct {
	StopOnFirstFailure   bool   `json:"stop_on_first_failure,omitempty"`
	MaxFailures          *int   `json:"max_failures,omitempty"`
	MaxTestsToRun        *int   `json:"max_tests_to_run,omitempty"`
	TestOrder            string `json:"test_order,omitempty"`
	CollectFailureDetail string `json:"collect_failure_artifacts,omitempty"`
}

type ValidatorConfig struct {
	Type string `json:"type,omitempty"`
}

func ParseExecutionPolicy(rawJSON string) ExecutionPolicy {
	trimmedJSON := strings.TrimSpace(rawJSON)
	if trimmedJSON == "" {
		return ExecutionPolicy{}
	}

	policy := ExecutionPolicy{}
	if err := json.Unmarshal([]byte(trimmedJSON), &policy); err != nil {
		return ExecutionPolicy{}
	}

	return normalizeExecutionPolicy(policy)
}

func SerializeExecutionPolicy(policy ExecutionPolicy) string {
	normalized := normalizeExecutionPolicy(policy)

	encodedJSON, err := json.Marshal(normalized)
	if err != nil {
		return "{}"
	}

	return string(encodedJSON)
}

func normalizeExecutionPolicy(policy ExecutionPolicy) ExecutionPolicy {
	order := strings.ToUpper(strings.TrimSpace(policy.TestOrder))
	if order == "" {
		order = ExecutionOrderFastFirst
	}

	collect := strings.ToUpper(strings.TrimSpace(policy.CollectFailureDetail))
	if collect == "" {
		collect = ExecutionArtifactsMinimal
	}

	return ExecutionPolicy{
		StopOnFirstFailure:   policy.StopOnFirstFailure,
		MaxFailures:          normalizeOptionalInt(policy.MaxFailures),
		MaxTestsToRun:        normalizeOptionalInt(policy.MaxTestsToRun),
		TestOrder:            order,
		CollectFailureDetail: collect,
	}
}

func DefaultExecutionPolicy(datasetType string) ExecutionPolicy {
	normalizedType := normalizeDatasetType(datasetType)

	maxFailures := 1
	if normalizedType == DatasetTypePublic {
		maxFailures = 2
	}

	return ExecutionPolicy{
		StopOnFirstFailure:   normalizedType == DatasetTypeHidden,
		MaxFailures:          &maxFailures,
		MaxTestsToRun:        nil,
		TestOrder:            ExecutionOrderFastFirst,
		CollectFailureDetail: ExecutionArtifactsMinimal,
	}
}

func DefaultValidatorConfig() ValidatorConfig {
	return ValidatorConfig{Type: ValidatorTypeJSONEquiv}
}

func isZeroExecutionPolicy(policy ExecutionPolicy) bool {
	if policy.StopOnFirstFailure {
		return false
	}
	if policy.MaxFailures != nil || policy.MaxTestsToRun != nil {
		return false
	}
	if strings.TrimSpace(policy.TestOrder) != "" {
		return false
	}
	if strings.TrimSpace(policy.CollectFailureDetail) != "" {
		return false
	}
	return true
}

func ParseValidatorConfig(rawJSON string) ValidatorConfig {
	trimmedJSON := strings.TrimSpace(rawJSON)
	if trimmedJSON == "" {
		return ValidatorConfig{}
	}

	config := ValidatorConfig{}
	if err := json.Unmarshal([]byte(trimmedJSON), &config); err != nil {
		return ValidatorConfig{}
	}

	return normalizeValidatorConfig(config)
}

func SerializeValidatorConfig(config ValidatorConfig) string {
	normalized := normalizeValidatorConfig(config)

	encodedJSON, err := json.Marshal(normalized)
	if err != nil {
		return "{}"
	}

	return string(encodedJSON)
}

func normalizeValidatorConfig(config ValidatorConfig) ValidatorConfig {
	return ValidatorConfig{Type: normalizeValidatorType(config.Type)}
}

func normalizeOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}

	normalized := *value
	if normalized < 0 {
		normalized = 0
	}

	return &normalized
}

func normalizeDatasetType(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case DatasetTypePublic, DatasetTypeHidden:
		return normalized
	default:
		return DatasetTypePublic
	}
}

func normalizeScoringMode(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case ScoringModeBinary, ScoringModePartial:
		return normalized
	default:
		return ScoringModeBinary
	}
}

func normalizeTestcaseVisibility(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case TestcaseVisibilityHidden, TestcaseVisibilityPublic:
		return normalized
	default:
		return TestcaseVisibilityPublic
	}
}

func normalizeTestcaseGroup(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case TestcaseGroupEdge, TestcaseGroupNormal, TestcaseGroupStress:
		return normalized
	default:
		return TestcaseGroupNormal
	}
}

func normalizeValidatorType(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case ValidatorTypeExact,
		ValidatorTypeJSONEquiv,
		ValidatorTypeUnorderedEquiv,
		ValidatorTypeFloatTolerance,
		ValidatorTypeCustomChecker:
		return normalized
	default:
		return ValidatorTypeExact
	}
}

func normalizeWeight(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}
