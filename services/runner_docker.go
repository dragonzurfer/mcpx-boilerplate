package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type DockerRunner struct {
	WorkDir      string
	DockerBin    string
	Image        string
	GoImage      string
	CImage       string
	CppImage     string
	JavaImage    string
	CPULimit     string
	TmpfsSizeMb  int
	PidsLimit    int
	CompileLimit time.Duration
	CompileMemMb int
}

func (r *DockerRunner) Run(input RunnerInput) (RunnerOutput, error) {
	if err := r.ensureDockerReady(); err != nil {
		return RunnerOutput{}, err
	}

	image := r.resolveDockerImageForLanguage(input.Language)
	if err := r.ensureDockerImageAvailable(image); err != nil {
		return RunnerOutput{}, err
	}

	config, err := buildLanguageConfig(languageConfigInput{Language: input.Language, Limits: input.Limits})
	if err != nil {
		return RunnerOutput{}, err
	}

	volumeName := buildJudgeVolumeName(input.SubmissionID)
	if err := createDockerVolume(r.resolveDockerBin(), volumeName); err != nil {
		return RunnerOutput{}, err
	}
	defer removeDockerVolume(r.resolveDockerBin(), volumeName)

	if err := r.writeSourceToVolume(dockerSourceInput{
		VolumeName: volumeName,
		Image:      image,
		SourceFile: config.SourceFile,
		SourceCode: input.SourceCode,
	}); err != nil {
		return RunnerOutput{}, err
	}

	compile := r.runCompile(dockerRunInput{
		SubmissionID:    input.SubmissionID,
		VolumeName:      volumeName,
		Image:           image,
		Command:         config.CompileCommand,
		Environment:     config.CompileEnv,
		TimeLimit:       r.resolveCompileTimeout(input.Limits.TimeLimitMs),
		MemoryLimit:     r.resolveCompileMemoryLimit(input.Limits.MemoryLimitKb),
		OutputLimit:     64 * 1024,
		CompilerVersion: config.CompilerVersion,
	})
	if !compile.OK {
		return RunnerOutput{Compile: compile}, nil
	}

	runOutput := r.runTests(dockerTestsInput{
		SubmissionID: input.SubmissionID,
		VolumeName:   volumeName,
		Image:        image,
		Config:       config,
		RunnerInput:  input,
	})

	return RunnerOutput{
		Compile:      compile,
		Runs:         runOutput.Results,
		StoppedEarly: runOutput.StoppedEarly,
		StopReason:   runOutput.StopReason,
	}, nil
}

type dockerTestsInput struct {
	SubmissionID uint
	VolumeName   string
	Image        string
	Config       languageConfig
	RunnerInput  RunnerInput
}

func (r *DockerRunner) runTests(input dockerTestsInput) runTestsOutput {
	results := make([]RunnerTestResult, 0, len(input.RunnerInput.Tests))
	failureCount := 0

	for _, test := range input.RunnerInput.Tests {
		result := r.runSingleTest(dockerTestInput{
			SubmissionID: input.SubmissionID,
			VolumeName:   input.VolumeName,
			Image:        input.Image,
			Config:       input.Config,
			Limits:       input.RunnerInput.Limits,
			Test:         test,
		})
		results = append(results, result)
		if isRunFailure(result) {
			failureCount++
		}

		decision := shouldStopEarly(stopEarlyInput{
			Policy:       input.RunnerInput.Policy,
			FailureCount: failureCount,
			Attempted:    len(results),
		})
		if decision.ShouldStop {
			return runTestsOutput{
				Results:      results,
				StoppedEarly: true,
				StopReason:   decision.Reason,
			}
		}
	}

	return runTestsOutput{Results: results}
}

type dockerTestInput struct {
	SubmissionID uint
	VolumeName   string
	Image        string
	Config       languageConfig
	Limits       ExecutionLimits
	Test         RunnerTestcase
}

func (r *DockerRunner) runSingleTest(input dockerTestInput) RunnerTestResult {
	commandOutput := r.runCommand(dockerRunInput{
		SubmissionID: input.SubmissionID,
		TestcaseID:   input.Test.TestcaseID,
		VolumeName:   input.VolumeName,
		Image:        input.Image,
		Command:      input.Config.RunCommand,
		Environment:  input.Config.RunEnv,
		Stdin:        input.Test.Input,
		TimeLimit:    resolveTimeLimit(input.Limits.TimeLimitMs),
		MemoryLimit:  resolveDockerMemoryLimit(input.Limits.MemoryLimitKb),
		OutputLimit:  resolveOutputLimitBytes(input.Limits.OutputLimitKb),
	})

	return RunnerTestResult{
		TestcaseID:      input.Test.TestcaseID,
		Stdout:          commandOutput.Stdout,
		Stderr:          commandOutput.Stderr,
		ExitCode:        commandOutput.ExitCode,
		TimeMs:          int(commandOutput.Duration.Milliseconds()),
		MemoryKb:        0,
		TimedOut:        commandOutput.TimedOut,
		MemoryExceeded:  commandOutput.MemoryExceeded,
		OutputTruncated: commandOutput.OutputTruncated,
	}
}

func (r *DockerRunner) runCompile(input dockerRunInput) RunnerCompileResult {
	if len(input.Command) == 0 {
		return RunnerCompileResult{OK: true}
	}

	commandOutput := r.runCommand(input)
	return RunnerCompileResult{
		OK:              commandOutput.ExitCode == 0 && !commandOutput.TimedOut && !commandOutput.MemoryExceeded,
		ExitCode:        commandOutput.ExitCode,
		Stderr:          commandOutput.Stderr,
		CompilerVersion: input.CompilerVersion,
		TimeMs:          int(commandOutput.Duration.Milliseconds()),
	}
}

type dockerRunInput struct {
	SubmissionID    uint
	TestcaseID      uint
	VolumeName      string
	Workspace       string
	Image           string
	Command         []string
	Environment     []string
	Stdin           string
	TimeLimit       time.Duration
	MemoryLimit     string
	OutputLimit     int
	CompilerVersion string
}

type dockerRunOutput struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	Duration        time.Duration
	TimedOut        bool
	MemoryExceeded  bool
	OutputTruncated bool
}

type dockerSourceInput struct {
	VolumeName string
	Image      string
	SourceFile string
	SourceCode string
}

func (r *DockerRunner) writeSourceToVolume(input dockerSourceInput) error {
	volumeName := strings.TrimSpace(input.VolumeName)
	if volumeName == "" {
		return errors.New("docker source volume is required")
	}

	sourceFile := strings.TrimSpace(input.SourceFile)
	if sourceFile == "" {
		return errors.New("source filename is required")
	}

	args := []string{
		"run",
		"--rm",
		"-i",
		"--workdir", "/workspace",
		"--network", "none",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"-v", volumeName + ":/workspace:rw",
		r.resolveDockerImage(input.Image),
		"sh", "-c", "cat > /workspace/" + sourceFile,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, r.resolveDockerBin(), args...)
	command.Stdin = strings.NewReader(input.SourceCode)

	stderr := &bytes.Buffer{}
	command.Stderr = stderr

	runErr := command.Run()
	if runErr == nil {
		return nil
	}

	trimmedStderr := strings.TrimSpace(stderr.String())
	if trimmedStderr == "" {
		return fmt.Errorf("failed to write source in docker volume: %w", runErr)
	}

	return fmt.Errorf("failed to write source in docker volume: %w: %s", runErr, trimmedStderr)
}

func (r *DockerRunner) runCommand(input dockerRunInput) dockerRunOutput {
	outputLimit := input.OutputLimit
	if outputLimit <= 0 {
		outputLimit = 64 * 1024
	}

	containerName := buildJudgeContainerName(input.SubmissionID, input.TestcaseID)
	args := r.buildDockerRunArgs(dockerArgsInput{
		ContainerName: containerName,
		VolumeName:    input.VolumeName,
		Workspace:     input.Workspace,
		Image:         input.Image,
		Command:       input.Command,
		Environment:   input.Environment,
		MemoryLimit:   input.MemoryLimit,
	})

	stdout := &limitedBuffer{limitBytes: outputLimit}
	stderr := &limitedBuffer{limitBytes: outputLimit}

	ctx, cancel := context.WithTimeout(context.Background(), input.TimeLimit)
	defer cancel()

	command := exec.CommandContext(ctx, r.resolveDockerBin(), args...)
	command.Stdin = strings.NewReader(input.Stdin)
	command.Stdout = stdout
	command.Stderr = stderr

	startedAt := time.Now().UTC()
	runErr := command.Run()
	duration := time.Since(startedAt)
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)

	memoryExceeded := false
	if !timedOut {
		memoryExceeded = inspectOOMKilled(r.resolveDockerBin(), containerName)
	}

	removeDockerContainer(r.resolveDockerBin(), containerName)

	stderrText := stderr.String()
	if strings.TrimSpace(stderrText) == "" && runErr != nil {
		stderrText = runErr.Error()
	}

	return dockerRunOutput{
		Stdout:          stdout.String(),
		Stderr:          stderrText,
		ExitCode:        resolveDockerExitCode(runErr, timedOut),
		Duration:        duration,
		TimedOut:        timedOut,
		MemoryExceeded:  memoryExceeded,
		OutputTruncated: stdout.Truncated() || stderr.Truncated(),
	}
}

type dockerArgsInput struct {
	ContainerName string
	VolumeName    string
	Workspace     string
	Image         string
	Command       []string
	Environment   []string
	MemoryLimit   string
}

func (r *DockerRunner) buildDockerRunArgs(input dockerArgsInput) []string {
	args := []string{
		"run",
		"-i",
		"--name", input.ContainerName,
		"--workdir", "/workspace",
		"--network", "none",
		"--read-only",
		"--tmpfs", buildTmpfsFlag(r.resolveTmpfsSizeMb()),
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--pids-limit", strconv.Itoa(r.resolvePidsLimit()),
		"--memory", input.MemoryLimit,
		"--memory-swap", input.MemoryLimit,
		"--cpus", r.resolveCPULimit(),
	}

	mountTarget := resolveDockerWorkspaceMount(input)
	if mountTarget != "" {
		args = append(args, "-v", mountTarget)
	}

	defaultEnv := []string{
		"HOME=/workspace",
		"TMPDIR=/workspace",
		"GOCACHE=/workspace/.cache/go-build",
	}

	for _, env := range append(defaultEnv, input.Environment...) {
		trimmed := strings.TrimSpace(env)
		if trimmed == "" {
			continue
		}
		args = append(args, "-e", trimmed)
	}

	args = append(args, r.resolveDockerImage(input.Image))
	args = append(args, input.Command...)
	return args
}

func resolveDockerWorkspaceMount(input dockerArgsInput) string {
	volumeName := strings.TrimSpace(input.VolumeName)
	if volumeName != "" {
		return volumeName + ":/workspace:rw"
	}

	workspace := strings.TrimSpace(input.Workspace)
	if workspace == "" {
		return ""
	}

	return workspace + ":/workspace:rw"
}

func buildJudgeContainerName(submissionID, testcaseID uint) string {
	now := time.Now().UTC().UnixNano()
	if testcaseID == 0 {
		return fmt.Sprintf("judge-%d-%d", submissionID, now)
	}
	return fmt.Sprintf("judge-%d-%d-%d", submissionID, testcaseID, now)
}

func buildJudgeVolumeName(submissionID uint) string {
	now := time.Now().UTC().UnixNano()
	return fmt.Sprintf("judge-vol-%d-%d", submissionID, now)
}

func buildTmpfsFlag(sizeMb int) string {
	return "/tmp:rw,nosuid,nodev,noexec,size=" + strconv.Itoa(sizeMb) + "m"
}

func resolveDockerExitCode(runErr error, timedOut bool) int {
	if timedOut {
		return -1
	}
	return exitCodeFromError(runErr)
}

func resolveDockerMemoryLimit(memoryLimitKb int) string {
	memoryBytes := resolveDockerMemoryBytes(memoryLimitKb, 128*1024*1024)
	return strconv.FormatInt(memoryBytes, 10)
}

func resolveDockerMemoryBytes(memoryLimitKb int, minimumBytes int64) int64 {
	if memoryLimitKb <= 0 {
		return maxInt64(minimumBytes, 256*1024*1024)
	}

	memoryBytes := int64(memoryLimitKb) * 1024
	return maxInt64(memoryBytes, minimumBytes)
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}

	return right
}

func inspectOOMKilled(dockerBin string, containerName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, dockerBin, "inspect", "-f", "{{.State.OOMKilled}}", containerName)
	output, err := command.Output()
	if err != nil {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(string(output)), "true")
}

func removeDockerContainer(dockerBin string, containerName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, dockerBin, "rm", "-f", containerName)
	_ = command.Run()
}

func createDockerVolume(dockerBin, volumeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, dockerBin, "volume", "create", volumeName)
	output, runErr := command.CombinedOutput()
	if runErr == nil {
		return nil
	}

	trimmedOutput := strings.TrimSpace(string(output))
	if trimmedOutput == "" {
		return fmt.Errorf("failed to create docker volume %s: %w", volumeName, runErr)
	}

	return fmt.Errorf("failed to create docker volume %s: %w: %s", volumeName, runErr, trimmedOutput)
}

func removeDockerVolume(dockerBin, volumeName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, dockerBin, "volume", "rm", "-f", volumeName)
	_ = command.Run()
}

func (r *DockerRunner) resolveDockerBin() string {
	binary := strings.TrimSpace(r.DockerBin)
	if binary == "" {
		return "docker"
	}
	return binary
}

func (r *DockerRunner) resolveDockerImage(value string) string {
	image := strings.TrimSpace(value)
	if image == "" {
		image = strings.TrimSpace(r.Image)
	}
	if image == "" {
		return "golang:1.25-bookworm"
	}
	return image
}

func (r *DockerRunner) resolveCPULimit() string {
	trimmed := strings.TrimSpace(r.CPULimit)
	if trimmed == "" {
		return "1"
	}
	return trimmed
}

func (r *DockerRunner) resolveTmpfsSizeMb() int {
	if r.TmpfsSizeMb <= 0 {
		return 64
	}
	return r.TmpfsSizeMb
}

func (r *DockerRunner) resolvePidsLimit() int {
	if r.PidsLimit <= 0 {
		return 128
	}
	return r.PidsLimit
}

func (r *DockerRunner) resolveCompileTimeout(timeLimitMs int) time.Duration {
	if r.CompileLimit > 0 {
		return r.CompileLimit
	}

	runtimeLimit := resolveTimeLimit(timeLimitMs)
	compileLimit := runtimeLimit * 4
	if compileLimit < 10*time.Second {
		return 10 * time.Second
	}
	if compileLimit > 120*time.Second {
		return 120 * time.Second
	}
	return compileLimit
}

func (r *DockerRunner) resolveCompileMemoryLimit(memoryLimitKb int) string {
	compileMemoryBytes := resolveDockerMemoryBytes(
		memoryLimitKb,
		r.resolveCompileMemoryFloorBytes(),
	)

	return strconv.FormatInt(compileMemoryBytes, 10)
}

func (r *DockerRunner) resolveCompileMemoryFloorBytes() int64 {
	compileMemoryMb := r.CompileMemMb
	if compileMemoryMb <= 0 {
		compileMemoryMb = 512
	}

	return int64(compileMemoryMb) * 1024 * 1024
}

func (r *DockerRunner) ensureDockerReady() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, r.resolveDockerBin(), "info")
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return fmt.Errorf("docker unavailable: %w", err)
	}
	return fmt.Errorf("docker unavailable: %w: %s", err, trimmed)
}

func (r *DockerRunner) ensureDockerImageAvailable(image string) error {
	resolvedImage := r.resolveDockerImage(image)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, r.resolveDockerBin(), "image", "inspect", resolvedImage)
	if err := command.Run(); err == nil {
		return nil
	}

	pullCtx, pullCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer pullCancel()

	pullCmd := exec.CommandContext(pullCtx, r.resolveDockerBin(), "pull", resolvedImage)
	output, err := pullCmd.CombinedOutput()
	if err == nil {
		return nil
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return fmt.Errorf("failed to pull judge image %s: %w", resolvedImage, err)
	}
	return fmt.Errorf("failed to pull judge image %s: %w: %s", resolvedImage, err, trimmed)
}

func (r *DockerRunner) resolveDockerImageForLanguage(language string) string {
	normalizedLanguage := strings.ToLower(strings.TrimSpace(language))
	if normalizedLanguage == "" {
		return r.resolveDockerImage("")
	}

	if normalizedLanguage == "go" || normalizedLanguage == "golang" {
		image := strings.TrimSpace(r.GoImage)
		if image != "" {
			return image
		}
		return "golang:1.25-bookworm"
	}

	if normalizedLanguage == "c" {
		image := strings.TrimSpace(r.CImage)
		if image != "" {
			return image
		}
		return "gcc:14-bookworm"
	}

	if normalizedLanguage == "cpp" || normalizedLanguage == "c++" {
		image := strings.TrimSpace(r.CppImage)
		if image != "" {
			return image
		}
		return "gcc:14-bookworm"
	}

	if normalizedLanguage == "java" {
		image := strings.TrimSpace(r.JavaImage)
		if image != "" {
			return image
		}
		return "eclipse-temurin:21-jdk"
	}

	return r.resolveDockerImage("")
}
