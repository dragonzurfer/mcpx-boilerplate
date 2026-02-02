package stores

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

func cacheKey(prefix string, input interface{}) string {
	raw, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	return prefix + ":" + string(raw)
}

func normalizeAccessLevels(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, strings.ToUpper(trimmed))
	}
	if len(normalized) == 0 {
		return nil
	}
	sort.Strings(normalized)
	return normalized
}

func normalizeString(value string) string {
	return strings.TrimSpace(value)
}

func timeBucket(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC().Truncate(time.Minute)
}
