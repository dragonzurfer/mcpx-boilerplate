package services

import (
	"errors"
	"strings"

	"github.com/mcpx/boilerplate/stores"
)

type ExecutionLimitInput struct {
	Problem  *stores.ProblemModel
	Language string
}

func ResolveExecutionLimits(input ExecutionLimitInput) (ExecutionLimits, error) {
	if input.Problem == nil {
		return ExecutionLimits{}, errors.New("problem missing")
	}

	constraints := stores.ParseProblemConstraints(input.Problem.ConstraintsJSON)
	limits := ExecutionLimits{
		TimeLimitMs:   constraints.TimeLimitMs,
		MemoryLimitKb: constraints.MemoryLimitKb,
		OutputLimitKb: constraints.OutputLimitKb,
	}

	languageKey := strings.ToLower(strings.TrimSpace(input.Language))
	if languageKey != "" {
		if langLimits, ok := constraints.Languages[languageKey]; ok {
			limits = applyLanguageOverrides(limits, langLimits)
		}
	}

	return normalizeLimits(limits), nil
}

func applyLanguageOverrides(base ExecutionLimits, lang stores.ProblemLangLimits) ExecutionLimits {
	if lang.TimeLimitMs > 0 {
		base.TimeLimitMs = lang.TimeLimitMs
	}
	if lang.MemoryLimitKb > 0 {
		base.MemoryLimitKb = lang.MemoryLimitKb
	}
	if lang.OutputLimitKb > 0 {
		base.OutputLimitKb = lang.OutputLimitKb
	}

	return base
}

func normalizeLimits(limits ExecutionLimits) ExecutionLimits {
	if limits.TimeLimitMs <= 0 {
		limits.TimeLimitMs = 1000
	}
	if limits.MemoryLimitKb <= 0 {
		limits.MemoryLimitKb = 262144
	}
	if limits.OutputLimitKb <= 0 {
		limits.OutputLimitKb = 1024
	}

	return limits
}
