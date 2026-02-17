package services

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mcpx/boilerplate/stores"
)

type LocalRunner struct {
	WorkDir string
}

type languageConfig struct {
	SourceFile      string
	CompileCommand  []string
	RunCommand      []string
	CompileEnv      []string
	RunEnv          []string
	CompilerVersion string
}

func (r *LocalRunner) Run(input RunnerInput) (RunnerOutput, error) {
	config, err := buildLanguageConfig(languageConfigInput{
		Language: input.Language,
		Limits:   input.Limits,
	})
	if err != nil {
		return RunnerOutput{}, err
	}

	workspace, err := r.createWorkspace(input.SubmissionID)
	if err != nil {
		return RunnerOutput{}, err
	}
	defer func() {
		_ = os.RemoveAll(workspace)
	}()

	sourcePath := filepath.Join(workspace, config.SourceFile)
	if err := os.WriteFile(sourcePath, []byte(input.SourceCode), 0o600); err != nil {
		return RunnerOutput{}, err
	}

	compileResult := runCompile(workspace, config)
	if !compileResult.OK {
		return RunnerOutput{Compile: compileResult}, nil
	}

	runOutput := runTests(runTestsInput{
		Workspace:   workspace,
		Config:      config,
		RunnerInput: input,
	})

	return RunnerOutput{
		Compile:      compileResult,
		Runs:         runOutput.Results,
		StoppedEarly: runOutput.StoppedEarly,
		StopReason:   runOutput.StopReason,
	}, nil
}

type languageConfigInput struct {
	Language string
	Limits   ExecutionLimits
}

func buildLanguageConfig(input languageConfigInput) (languageConfig, error) {
	language := strings.ToLower(strings.TrimSpace(input.Language))

	switch language {
	case "go", "golang":
		return goConfig(), nil
	case "c":
		return cConfig(), nil
	case "cpp", "c++":
		return cppConfig(), nil
	case "java":
		return javaConfig(input.Limits), nil
	default:
		return languageConfig{}, errors.New("unsupported language")
	}
}

func goConfig() languageConfig {
	return languageConfig{
		SourceFile:      "main.go",
		CompileCommand:  []string{"go", "build", "-o", "main", "main.go"},
		RunCommand:      []string{"./main"},
		CompileEnv:      []string{"GO111MODULE=off"},
		CompilerVersion: "go",
	}
}

func cConfig() languageConfig {
	return languageConfig{
		SourceFile:      "main.c",
		CompileCommand:  []string{"gcc", "-O2", "-std=c11", "-o", "main", "main.c"},
		RunCommand:      []string{"./main"},
		CompilerVersion: "gcc",
	}
}

func cppConfig() languageConfig {
	return languageConfig{
		SourceFile:      "main.cpp",
		CompileCommand:  []string{"g++", "-O2", "-std=c++17", "-o", "main", "main.cpp"},
		RunCommand:      []string{"./main"},
		CompilerVersion: "g++",
	}
}

func javaConfig(limits ExecutionLimits) languageConfig {
	memoryLimit := resolveJavaMemoryLimit(limits.MemoryLimitKb)

	return languageConfig{
		SourceFile:      "Main.java",
		CompileCommand:  []string{"javac", "Main.java"},
		RunCommand:      []string{"java", "-Xmx" + memoryLimit, "Main"},
		CompilerVersion: "javac",
	}
}

func resolveJavaMemoryLimit(limitKb int) string {
	if limitKb <= 0 {
		return "512m"
	}

	limitMb := limitKb / 1024
	if limitMb < 128 {
		limitMb = 128
	}

	return strconv.Itoa(limitMb) + "m"
}

func (r *LocalRunner) createWorkspace(submissionID uint) (string, error) {
	base := strings.TrimSpace(r.WorkDir)
	if base == "" {
		return os.MkdirTemp("", "judge-")
	}

	prefix := "judge-"
	if submissionID != 0 {
		prefix = prefix + strconv.FormatUint(uint64(submissionID), 10) + "-"
	}

	return os.MkdirTemp(base, prefix)
}

func runCompile(workspace string, config languageConfig) RunnerCompileResult {
	if len(config.CompileCommand) == 0 {
		return RunnerCompileResult{OK: true}
	}

	cmd := exec.Command(config.CompileCommand[0], config.CompileCommand[1:]...)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), config.CompileEnv...)

	stderr := &limitedBuffer{limitBytes: 64 * 1024}
	cmd.Stdout = stderr
	cmd.Stderr = stderr

	start := time.Now().UTC()
	err := cmd.Run()
	elapsed := time.Since(start)

	exitCode := exitCodeFromError(err)
	return RunnerCompileResult{
		OK:              err == nil,
		ExitCode:        exitCode,
		Stderr:          stderr.String(),
		CompilerVersion: config.CompilerVersion,
		TimeMs:          int(elapsed.Milliseconds()),
	}
}

type runTestsInput struct {
	Workspace   string
	Config      languageConfig
	RunnerInput RunnerInput
}

type runTestsOutput struct {
	Results      []RunnerTestResult
	StoppedEarly bool
	StopReason   string
}

func runTests(input runTestsInput) runTestsOutput {
	runResults := make([]RunnerTestResult, 0, len(input.RunnerInput.Tests))
	failureCount := 0

	for _, test := range input.RunnerInput.Tests {
		runResult := runTestCase(runTestInput{
			Workspace: input.Workspace,
			Config:    input.Config,
			Limits:    input.RunnerInput.Limits,
			Test:      test,
		})

		runResults = append(runResults, runResult)

		if isRunFailure(runResult) {
			failureCount++
		}

		decision := shouldStopEarly(stopEarlyInput{
			Policy:       input.RunnerInput.Policy,
			FailureCount: failureCount,
			Attempted:    len(runResults),
		})
		if decision.ShouldStop {
			return runTestsOutput{
				Results:      runResults,
				StoppedEarly: true,
				StopReason:   decision.Reason,
			}
		}
	}

	return runTestsOutput{Results: runResults}
}

type runTestInput struct {
	Workspace string
	Config    languageConfig
	Limits    ExecutionLimits
	Test      RunnerTestcase
}

func runTestCase(input runTestInput) RunnerTestResult {
	outputLimit := resolveOutputLimitBytes(input.Limits.OutputLimitKb)
	timeLimit := resolveTimeLimit(input.Limits.TimeLimitMs)

	commandOutput := runTestCommand(commandRunInput{
		Workspace:   input.Workspace,
		Config:      input.Config,
		Input:       input.Test.Input,
		TimeLimit:   timeLimit,
		OutputLimit: outputLimit,
	})

	return RunnerTestResult{
		TestcaseID:      input.Test.TestcaseID,
		Stdout:          commandOutput.Stdout,
		Stderr:          commandOutput.Stderr,
		ExitCode:        commandOutput.ExitCode,
		TimeMs:          int(commandOutput.Duration.Milliseconds()),
		MemoryKb:        0,
		TimedOut:        commandOutput.TimedOut,
		MemoryExceeded:  false,
		OutputTruncated: commandOutput.OutputTruncated,
	}
}

type commandRunInput struct {
	Workspace   string
	Config      languageConfig
	Input       string
	TimeLimit   time.Duration
	OutputLimit int
}

type commandRunOutput struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	TimedOut        bool
	OutputTruncated bool
	Duration        time.Duration
}

func runTestCommand(input commandRunInput) commandRunOutput {
	stdout := &limitedBuffer{limitBytes: input.OutputLimit}
	stderr := &limitedBuffer{limitBytes: input.OutputLimit}

	ctx, cancel := context.WithTimeout(context.Background(), input.TimeLimit)
	defer cancel()

	command := exec.CommandContext(ctx, input.Config.RunCommand[0], input.Config.RunCommand[1:]...)
	command.Dir = input.Workspace
	command.Env = append(os.Environ(), input.Config.RunEnv...)
	command.Stdin = strings.NewReader(input.Input)
	command.Stdout = stdout
	command.Stderr = stderr

	start := time.Now().UTC()
	err := command.Run()
	elapsed := time.Since(start)

	return commandRunOutput{
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		ExitCode:        exitCodeFromError(err),
		TimedOut:        errors.Is(ctx.Err(), context.DeadlineExceeded),
		OutputTruncated: stdout.Truncated() || stderr.Truncated(),
		Duration:        elapsed,
	}
}

func resolveTimeLimit(limitMs int) time.Duration {
	if limitMs <= 0 {
		return 2 * time.Second
	}

	return time.Duration(limitMs) * time.Millisecond
}

func resolveOutputLimitBytes(limitKb int) int {
	if limitKb <= 0 {
		return 256 * 1024
	}

	return limitKb * 1024
}

type stopEarlyInput struct {
	Policy       stores.ExecutionPolicy
	FailureCount int
	Attempted    int
}

type stopDecision struct {
	ShouldStop bool
	Reason     string
}

func shouldStopEarly(input stopEarlyInput) stopDecision {
	if input.Policy.StopOnFirstFailure && input.FailureCount > 0 {
		return stopDecision{ShouldStop: true, Reason: "FIRST_FAILURE"}
	}

	if input.Policy.MaxFailures != nil && input.FailureCount >= *input.Policy.MaxFailures {
		return stopDecision{ShouldStop: true, Reason: "MAX_FAILURES"}
	}

	if input.Policy.MaxTestsToRun != nil && input.Attempted >= *input.Policy.MaxTestsToRun {
		return stopDecision{ShouldStop: true, Reason: "MAX_TESTS"}
	}

	return stopDecision{}
}

func isRunFailure(result RunnerTestResult) bool {
	if result.TimedOut {
		return true
	}
	if result.MemoryExceeded {
		return true
	}
	if result.OutputTruncated {
		return true
	}
	return result.ExitCode != 0
}

type limitedBuffer struct {
	buf        bytes.Buffer
	limitBytes int
	truncated  bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limitBytes - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}

	if len(p) > remaining {
		b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}

	return b.buf.Write(p)
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}

func (b *limitedBuffer) Truncated() bool {
	return b.truncated
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}

	exitErr := &exec.ExitError{}
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
}
