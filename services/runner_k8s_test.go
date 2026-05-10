package services

import (
	"testing"
	"time"
)

func TestResolveK8sMemoryLimitMinimum(t *testing.T) {
	memoryLimit := resolveK8sMemoryLimit(1)
	if memoryLimit != "128Mi" {
		t.Fatalf("expected minimum memory limit 128Mi, got %s", memoryLimit)
	}
}

func TestResolveK8sJobTimeoutBounds(t *testing.T) {
	minimumTimeout := resolveK8sJobTimeout(k8sJobTimeoutInput{
		ExecutionLimits: ExecutionLimits{TimeLimitMs: 10},
		TestCount:       1,
	})
	if minimumTimeout != 30*time.Second {
		t.Fatalf("expected minimum timeout 30s, got %s", minimumTimeout)
	}

	maximumTimeout := resolveK8sJobTimeout(k8sJobTimeoutInput{
		ExecutionLimits: ExecutionLimits{TimeLimitMs: 30000},
		TestCount:       100,
	})
	if maximumTimeout != 10*time.Minute {
		t.Fatalf("expected maximum timeout 10m, got %s", maximumTimeout)
	}
}

func TestParseJudgeWorkerResult(t *testing.T) {
	rawResult := []byte(`{"runner_output":{"compile":{"ok":true}}}`)

	workerResult, err := parseJudgeWorkerResult(rawResult)
	if err != nil {
		t.Fatalf("expected parse success, got error: %v", err)
	}
	if workerResult.RunnerOutput == nil {
		t.Fatalf("expected runner output in parsed result")
	}
}
