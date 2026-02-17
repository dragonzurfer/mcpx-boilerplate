package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/mcpx/boilerplate/stores"
)

type OutputValidationInput struct {
	Validator stores.ValidatorConfig
	Expected  string
	Actual    string
}

func ValidateOutput(input OutputValidationInput) (bool, error) {
	validatorType := strings.ToUpper(strings.TrimSpace(input.Validator.Type))
	switch validatorType {
	case stores.ValidatorTypeJSONEquiv:
		return jsonEquivalent(input.Expected, input.Actual)
	case stores.ValidatorTypeUnorderedEquiv:
		return unorderedJSONEquivalent(input.Expected, input.Actual)
	case stores.ValidatorTypeFloatTolerance:
		return floatEquivalent(input.Expected, input.Actual)
	case stores.ValidatorTypeCustomChecker:
		return false, errors.New("custom checker not supported")
	case stores.ValidatorTypeExact, "":
		return exactMatch(input.Expected, input.Actual), nil
	default:
		return exactMatch(input.Expected, input.Actual), nil
	}
}

func exactMatch(expected string, actual string) bool {
	return normalizeOutput(expected) == normalizeOutput(actual)
}

func normalizeOutput(value string) string {
	trimmed := strings.TrimSpace(value)
	return strings.ReplaceAll(trimmed, "\r\n", "\n")
}

func jsonEquivalent(expected string, actual string) (bool, error) {
	expectedValue := interface{}(nil)
	actualValue := interface{}(nil)

	if err := json.Unmarshal([]byte(strings.TrimSpace(expected)), &expectedValue); err != nil {
		return exactMatch(expected, actual), nil
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(actual)), &actualValue); err != nil {
		return exactMatch(expected, actual), nil
	}

	return reflect.DeepEqual(expectedValue, actualValue), nil
}

func unorderedJSONEquivalent(expected string, actual string) (bool, error) {
	expectedList, err := parseJSONArray(expected)
	if err != nil {
		return exactMatch(expected, actual), nil
	}
	actualList, err := parseJSONArray(actual)
	if err != nil {
		return exactMatch(expected, actual), nil
	}

	normalizedExpected := normalizeJSONArray(expectedList)
	normalizedActual := normalizeJSONArray(actualList)

	return reflect.DeepEqual(normalizedExpected, normalizedActual), nil
}

func parseJSONArray(raw string) ([]interface{}, error) {
	trimmed := strings.TrimSpace(raw)
	parsed := []interface{}{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func normalizeJSONArray(values []interface{}) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			encoded = []byte(fmt.Sprint(value))
		}
		normalized = append(normalized, string(encoded))
	}

	sort.Strings(normalized)
	return normalized
}

func floatEquivalent(expected string, actual string) (bool, error) {
	left, err := parseFloatValue(expected)
	if err != nil {
		return exactMatch(expected, actual), nil
	}
	right, err := parseFloatValue(actual)
	if err != nil {
		return exactMatch(expected, actual), nil
	}

	delta := left - right
	if delta < 0 {
		delta = -delta
	}
	return delta <= 1e-6, nil
}

func parseFloatValue(raw string) (float64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, errors.New("empty float")
	}

	var parsed float64
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed, nil
	}

	return strconv.ParseFloat(trimmed, 64)
}
