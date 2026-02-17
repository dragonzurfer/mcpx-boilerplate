package services

import "github.com/mcpx/boilerplate/stores"

type ExecutionLimits struct {
	TimeLimitMs   int `json:"time_limit_ms"`
	MemoryLimitKb int `json:"memory_limit_kb"`
	OutputLimitKb int `json:"output_limit_kb"`
}

type JudgeOverall struct {
	Verdict      string `json:"verdict"`
	Passed       int    `json:"passed"`
	Attempted    int    `json:"attempted"`
	Total        int    `json:"total"`
	StoppedEarly bool   `json:"stopped_early"`
	StopReason   string `json:"stop_reason,omitempty"`
	RuntimeMs    int    `json:"runtime_ms"`
	MemoryKb     int    `json:"memory_kb"`
}

type JudgeCompile struct {
	Verdict         string `json:"verdict"`
	ExitCode        int    `json:"exit_code"`
	Stderr          string `json:"stderr"`
	CompilerVersion string `json:"compiler_version"`
	TimeMs          int    `json:"time_ms"`
}

type JudgeTestDetail struct {
	Input        string `json:"input,omitempty"`
	Expected     string `json:"expected,omitempty"`
	Actual       string `json:"actual,omitempty"`
	ExpectedHash string `json:"expected_hash,omitempty"`
	ActualHash   string `json:"actual_hash,omitempty"`
}

type JudgeTestResult struct {
	TestcaseID uint             `json:"testcase_id"`
	Verdict    string           `json:"verdict"`
	RuntimeMs  int              `json:"runtime_ms"`
	MemoryKb   int              `json:"memory_kb"`
	Detail     *JudgeTestDetail `json:"detail,omitempty"`
}

type JudgeArtifacts struct {
	StdoutTruncated bool   `json:"stdout_truncated"`
	StderrTruncated bool   `json:"stderr_truncated"`
	LogsRef         string `json:"logs_ref,omitempty"`
}

type JudgeTiming struct {
	QueueMs   int `json:"queue_ms"`
	CompileMs int `json:"compile_ms"`
	RunMs     int `json:"run_ms"`
}

type JudgeResult struct {
	SubmissionID uint              `json:"submission_id"`
	Overall      JudgeOverall      `json:"overall"`
	Compile      JudgeCompile      `json:"compile"`
	Tests        []JudgeTestResult `json:"tests"`
	Artifacts    JudgeArtifacts    `json:"artifacts"`
	Timing       JudgeTiming       `json:"timing"`
}

type RunnerInput struct {
	SubmissionID uint
	Language     string
	SourceCode   string
	Limits       ExecutionLimits
	Policy       stores.ExecutionPolicy
	Tests        []RunnerTestcase
}

type RunnerTestcase struct {
	TestcaseID uint
	Input      string
}

type RunnerCompileResult struct {
	OK              bool
	ExitCode        int
	Stderr          string
	CompilerVersion string
	TimeMs          int
}

type RunnerTestResult struct {
	TestcaseID      uint
	Stdout          string
	Stderr          string
	ExitCode        int
	TimeMs          int
	MemoryKb        int
	TimedOut        bool
	MemoryExceeded  bool
	OutputTruncated bool
}

type RunnerOutput struct {
	Compile      RunnerCompileResult
	Runs         []RunnerTestResult
	StoppedEarly bool
	StopReason   string
}

type Runner interface {
	Run(input RunnerInput) (RunnerOutput, error)
}
