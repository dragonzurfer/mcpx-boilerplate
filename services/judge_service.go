package services

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/mcpx/boilerplate/stores"
)

type JudgeService struct {
	Store  *stores.Store
	Runner Runner
	Logger Logger
}

type Logger interface {
	Printf(format string, v ...interface{})
}

func (s *JudgeService) RunOnce() (bool, error) {
	runner := s.resolveRunner()
	if runner == nil {
		return false, errors.New("runner unavailable")
	}

	submission, err := s.Store.ClaimNextQueuedSubmission(stores.SubmissionClaimInput{Now: time.Now().UTC()})
	if err != nil {
		if errors.Is(err, stores.ErrNoQueuedSubmission) {
			return false, nil
		}
		return false, err
	}

	err = s.processSubmission(judgeSubmissionInput{Submission: submission, Runner: runner})
	if err != nil {
		return false, err
	}
	return true, nil
}

type judgeSubmissionInput struct {
	Submission *stores.SubmissionModel
	Runner     Runner
}

func (s *JudgeService) processSubmission(input judgeSubmissionInput) error {
	context, err := s.loadJudgeContext(judgeContextInput{Submission: input.Submission})
	if err != nil {
		return s.failSubmission(input.Submission, err)
	}

	return s.runAndFinalize(input.Runner, context)
}

func (s *JudgeService) runAndFinalize(runner Runner, context judgeContext) error {
	runnerInput := buildRunnerInput(context)
	runnerOutput, err := runner.Run(runnerInput)
	if err != nil {
		return s.failSubmission(context.Submission, err)
	}

	judgeResult := buildJudgeResult(context, runnerOutput)
	if err := s.persistJudgeResult(judgeResult, context.Submission); err != nil {
		return s.failSubmission(context.Submission, err)
	}

	return s.markSubmissionCompleted(context.Submission)
}

func (s *JudgeService) markSubmissionCompleted(submission *stores.SubmissionModel) error {
	finishedAt := time.Now().UTC()
	updateInput := stores.SubmissionStatusUpdateInput{
		SubmissionID: submission.ID,
		Status:       stores.SubmissionStatusCompleted,
		FinishedAt:   &finishedAt,
	}
	return s.Store.UpdateSubmissionStatus(updateInput)
}

type judgeContext struct {
	Submission       *stores.SubmissionModel
	Problem          *stores.ProblemModel
	Dataset          *stores.DatasetModel
	Testcases        []stores.TestcaseModel
	Policy           stores.ExecutionPolicy
	Limits           ExecutionLimits
	DatasetType      string
	ValidatorDefault stores.ValidatorConfig
}

type judgeContextInput struct {
	Submission *stores.SubmissionModel
}

type judgeConfigInput struct {
	Problem    *stores.ProblemModel
	Dataset    *stores.DatasetModel
	Submission *stores.SubmissionModel
}

type judgeConfig struct {
	Policy           stores.ExecutionPolicy
	Limits           ExecutionLimits
	DatasetType      string
	ValidatorDefault stores.ValidatorConfig
}

func (s *JudgeService) loadJudgeContext(input judgeContextInput) (judgeContext, error) {
	if input.Submission == nil {
		return judgeContext{}, errors.New("submission missing")
	}

	problem, err := s.loadProblem(input.Submission)
	if err != nil {
		return judgeContext{}, err
	}

	dataset, err := s.loadDataset(input.Submission)
	if err != nil {
		return judgeContext{}, err
	}

	testcases, err := s.loadTestcases(dataset)
	if err != nil {
		return judgeContext{}, err
	}

	config, err := resolveJudgeConfig(judgeConfigInput{
		Problem:    problem,
		Dataset:    dataset,
		Submission: input.Submission,
	})
	if err != nil {
		return judgeContext{}, err
	}

	return judgeContext{
		Submission:       input.Submission,
		Problem:          problem,
		Dataset:          dataset,
		Testcases:        testcases,
		Policy:           config.Policy,
		Limits:           config.Limits,
		DatasetType:      config.DatasetType,
		ValidatorDefault: config.ValidatorDefault,
	}, nil
}

func resolveJudgeConfig(input judgeConfigInput) (judgeConfig, error) {
	policy := stores.ParseExecutionPolicy(input.Dataset.ExecutionPolicyJSON)
	validatorDefault := stores.ParseValidatorConfig(input.Dataset.ValidatorDefaultJSON)

	limits, err := ResolveExecutionLimits(ExecutionLimitInput{
		Problem:  input.Problem,
		Language: input.Submission.Language,
	})
	if err != nil {
		return judgeConfig{}, err
	}

	if err := ensureIOSupported(input.Problem); err != nil {
		return judgeConfig{}, err
	}

	resolvedPolicy := resolveExecutionPolicy(policyResolveInput{
		DatasetPolicy:  policy,
		DatasetType:    input.Dataset.Type,
		SubmissionMode: input.Submission.Mode,
	})

	datasetType := strings.ToUpper(strings.TrimSpace(input.Dataset.Type))
	return judgeConfig{
		Policy:           resolvedPolicy,
		Limits:           limits,
		DatasetType:      datasetType,
		ValidatorDefault: validatorDefault,
	}, nil
}

func (s *JudgeService) loadProblem(submission *stores.SubmissionModel) (*stores.ProblemModel, error) {
	return s.Store.GetProblemByID(stores.ProblemByIDLookupInput{ProblemID: submission.ProblemID})
}

func (s *JudgeService) loadDataset(submission *stores.SubmissionModel) (*stores.DatasetModel, error) {
	return s.Store.GetDatasetByID(stores.DatasetLookupInput{DatasetID: submission.DatasetID})
}

func (s *JudgeService) loadTestcases(dataset *stores.DatasetModel) ([]stores.TestcaseModel, error) {
	output, err := s.Store.ListTestcases(stores.TestcaseListInput{DatasetID: dataset.ID})
	if err != nil {
		return nil, err
	}
	return output.Testcases, nil
}

func ensureIOSupported(problem *stores.ProblemModel) error {
	ioSpec := stores.ParseProblemIOSpec(problem.IOSpecJSON)
	mode := strings.ToUpper(strings.TrimSpace(ioSpec.Mode))
	if mode == "" || mode == "STDIN" {
		return nil
	}
	return errors.New("io spec mode not supported yet")
}

type policyResolveInput struct {
	DatasetPolicy  stores.ExecutionPolicy
	DatasetType    string
	SubmissionMode string
}

func resolveExecutionPolicy(input policyResolveInput) stores.ExecutionPolicy {
	policy := input.DatasetPolicy
	mode := strings.ToUpper(strings.TrimSpace(input.SubmissionMode))
	datasetType := strings.ToUpper(strings.TrimSpace(input.DatasetType))

	if datasetType == stores.DatasetTypeHidden || mode == stores.SubmissionModeSubmit {
		maxFailures := 1
		policy.StopOnFirstFailure = true
		policy.MaxFailures = &maxFailures
	}

	return policy
}

func buildRunnerInput(context judgeContext) RunnerInput {
	ordered := orderTestcases(testcaseOrderInput{
		Policy:    context.Policy,
		Testcases: context.Testcases,
	})

	return RunnerInput{
		SubmissionID: context.Submission.ID,
		Language:     context.Submission.Language,
		SourceCode:   context.Submission.CodeText,
		Limits:       context.Limits,
		Policy:       context.Policy,
		Tests:        buildRunnerTests(ordered),
	}
}

func buildRunnerTests(testcases []stores.TestcaseModel) []RunnerTestcase {
	items := make([]RunnerTestcase, 0, len(testcases))
	for _, testcase := range testcases {
		items = append(items, RunnerTestcase{TestcaseID: testcase.ID, Input: testcase.InputText})
	}
	return items
}

type testcaseOrderInput struct {
	Policy    stores.ExecutionPolicy
	Testcases []stores.TestcaseModel
}

func orderTestcases(input testcaseOrderInput) []stores.TestcaseModel {
	if strings.ToUpper(strings.TrimSpace(input.Policy.TestOrder)) != stores.ExecutionOrderFastFirst {
		return input.Testcases
	}

	ordered := make([]stores.TestcaseModel, len(input.Testcases))
	copy(ordered, input.Testcases)

	sort.SliceStable(ordered, func(i, j int) bool {
		left := testcaseGroupRank(ordered[i].Group)
		right := testcaseGroupRank(ordered[j].Group)

		if left != right {
			return left < right
		}
		if ordered[i].Position != ordered[j].Position {
			return ordered[i].Position < ordered[j].Position
		}
		return ordered[i].ID < ordered[j].ID
	})

	return ordered
}

func testcaseGroupRank(value string) int {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case stores.TestcaseGroupEdge:
		return 0
	case stores.TestcaseGroupStress:
		return 2
	default:
		return 1
	}
}

func buildJudgeResult(context judgeContext, runnerOutput RunnerOutput) JudgeResult {
	compile := buildCompileResult(runnerOutput.Compile)
	testResults := buildTestResults(context, runnerOutput.Runs)

	overall := buildOverallResult(overallInput{
		CompileVerdict: compile.Verdict,
		TestResults:    testResults,
		TotalTests:     len(context.Testcases),
		StoppedEarly:   runnerOutput.StoppedEarly,
		StopReason:     runnerOutput.StopReason,
	})

	timing := buildTiming(timingInput{
		Submission: context.Submission,
		CompileMs:  compile.TimeMs,
		Tests:      testResults,
	})
	artifacts := buildArtifacts(runnerOutput.Runs)

	return JudgeResult{
		SubmissionID: context.Submission.ID,
		Overall:      overall,
		Compile:      compile,
		Tests:        testResults,
		Artifacts:    artifacts,
		Timing:       timing,
	}
}

type overallInput struct {
	CompileVerdict string
	TestResults    []JudgeTestResult
	TotalTests     int
	StoppedEarly   bool
	StopReason     string
}

func buildOverallResult(input overallInput) JudgeOverall {
	if input.CompileVerdict == stores.VerdictCompileError {
		return JudgeOverall{
			Verdict:      stores.VerdictCompileError,
			Passed:       0,
			Attempted:    0,
			Total:        input.TotalTests,
			StoppedEarly: input.StoppedEarly,
			StopReason:   input.StopReason,
			RuntimeMs:    0,
			MemoryKb:     0,
		}
	}

	passed := 0
	attempted := len(input.TestResults)
	verdict := stores.VerdictAccepted
	maxMemory := 0
	runtime := 0

	for _, test := range input.TestResults {
		runtime += test.RuntimeMs
		if test.MemoryKb > maxMemory {
			maxMemory = test.MemoryKb
		}
		if test.Verdict == stores.VerdictAccepted {
			passed++
			continue
		}
		if verdict == stores.VerdictAccepted {
			verdict = test.Verdict
		}
	}

	return JudgeOverall{
		Verdict:      verdict,
		Passed:       passed,
		Attempted:    attempted,
		Total:        input.TotalTests,
		StoppedEarly: input.StoppedEarly,
		StopReason:   input.StopReason,
		RuntimeMs:    runtime,
		MemoryKb:     maxMemory,
	}
}

func buildCompileResult(result RunnerCompileResult) JudgeCompile {
	verdict := stores.VerdictAccepted
	if !result.OK {
		verdict = stores.VerdictCompileError
	}

	return JudgeCompile{
		Verdict:         verdict,
		ExitCode:        result.ExitCode,
		Stderr:          result.Stderr,
		CompilerVersion: result.CompilerVersion,
		TimeMs:          result.TimeMs,
	}
}

func buildTestResults(context judgeContext, runs []RunnerTestResult) []JudgeTestResult {
	results := make([]JudgeTestResult, 0, len(runs))
	testcaseLookup := make(map[uint]stores.TestcaseModel, len(context.Testcases))

	for _, testcase := range context.Testcases {
		testcaseLookup[testcase.ID] = testcase
	}

	for _, run := range runs {
		testcase := testcaseLookup[run.TestcaseID]
		validator := resolveValidator(context.ValidatorDefault, testcase.ValidatorOverrideJSON)
		result := evaluateTestResult(testResultInput{
			Run:            run,
			Testcase:       testcase,
			Validator:      validator,
			DatasetType:    context.DatasetType,
			SubmissionMode: context.Submission.Mode,
		})
		results = append(results, result)
	}

	return results
}

type testResultInput struct {
	Run            RunnerTestResult
	Testcase       stores.TestcaseModel
	Validator      stores.ValidatorConfig
	DatasetType    string
	SubmissionMode string
}

func evaluateTestResult(input testResultInput) JudgeTestResult {
	verdict := mapRunVerdict(input.Run)

	detail := buildDetailIfAllowed(input, verdict)

	if verdict == stores.VerdictAccepted {
		matched, err := ValidateOutput(OutputValidationInput{Validator: input.Validator, Expected: input.Testcase.ExpectedText, Actual: input.Run.Stdout})
		if err != nil {
			verdict = stores.VerdictInternalError
		} else if !matched {
			verdict = stores.VerdictWrongAnswer
		}
		if detail != nil && detail.Actual == "" {
			detail.Actual = input.Run.Stdout
		}
	}

	return JudgeTestResult{
		TestcaseID: input.Run.TestcaseID,
		Verdict:    verdict,
		RuntimeMs:  input.Run.TimeMs,
		MemoryKb:   input.Run.MemoryKb,
		Detail:     detail,
	}
}

func mapRunVerdict(run RunnerTestResult) string {
	if run.TimedOut {
		return stores.VerdictTLE
	}
	if run.MemoryExceeded {
		return stores.VerdictMLE
	}
	if run.OutputTruncated {
		return stores.VerdictOutputLimitExceeded
	}
	if run.ExitCode != 0 {
		return stores.VerdictRuntimeError
	}
	return stores.VerdictAccepted
}

func buildDetailIfAllowed(input testResultInput, verdict string) *JudgeTestDetail {
	visibilityInput := detailVisibilityInput{
		DatasetType:    input.DatasetType,
		SubmissionMode: input.SubmissionMode,
		Visibility:     input.Testcase.Visibility,
	}
	if shouldHideTestDetail(visibilityInput) {
		return buildHashedDetail(input.Testcase.ExpectedText, input.Run.Stdout)
	}

	return &JudgeTestDetail{
		Input:    input.Testcase.InputText,
		Expected: input.Testcase.ExpectedText,
		Actual:   input.Run.Stdout,
	}
}

func buildHashedDetail(expected string, actual string) *JudgeTestDetail {
	return &JudgeTestDetail{
		ExpectedHash: HashString(expected),
		ActualHash:   HashString(actual),
	}
}

type detailVisibilityInput struct {
	DatasetType    string
	SubmissionMode string
	Visibility     string
}

func shouldHideTestDetail(input detailVisibilityInput) bool {
	if strings.ToUpper(strings.TrimSpace(input.DatasetType)) == stores.DatasetTypeHidden {
		return true
	}
	if strings.ToUpper(strings.TrimSpace(input.SubmissionMode)) == stores.SubmissionModeSubmit {
		return true
	}
	if strings.ToUpper(strings.TrimSpace(input.Visibility)) == stores.TestcaseVisibilityHidden {
		return true
	}
	return false
}

func resolveValidator(defaultValidator stores.ValidatorConfig, overrideJSON string) stores.ValidatorConfig {
	override := stores.ParseValidatorConfig(overrideJSON)
	if strings.TrimSpace(override.Type) == "" {
		return defaultValidator
	}
	return override
}

type timingInput struct {
	Submission *stores.SubmissionModel
	CompileMs  int
	Tests      []JudgeTestResult
}

func buildTiming(input timingInput) JudgeTiming {
	queueMs := 0
	if input.Submission.StartedAt != nil {
		queueMs = int(input.Submission.StartedAt.Sub(input.Submission.QueuedAt).Milliseconds())
	}

	runMs := 0
	for _, test := range input.Tests {
		runMs += test.RuntimeMs
	}

	return JudgeTiming{
		QueueMs:   queueMs,
		CompileMs: input.CompileMs,
		RunMs:     runMs,
	}
}

func buildArtifacts(runs []RunnerTestResult) JudgeArtifacts {
	stdoutTruncated := false
	stderrTruncated := false

	for _, run := range runs {
		if run.OutputTruncated {
			stdoutTruncated = true
			stderrTruncated = true
			break
		}
	}

	return JudgeArtifacts{
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
	}
}

func (s *JudgeService) persistJudgeResult(result JudgeResult, submission *stores.SubmissionModel) error {
	overallJSON := mustMarshalJSON(result.Overall)
	compileJSON := mustMarshalJSON(result.Compile)
	testsJSON := mustMarshalJSON(result.Tests)
	timingJSON := mustMarshalJSON(result.Timing)
	artifactsJSON := mustMarshalJSON(result.Artifacts)
	receiptJSON := s.buildReceipt(result)

	_, err := s.Store.UpsertSubmissionResult(stores.SubmissionResultUpsertInput{
		SubmissionID:  submission.ID,
		OverallJSON:   overallJSON,
		CompileJSON:   compileJSON,
		TestsJSON:     testsJSON,
		TimingJSON:    timingJSON,
		ArtifactsJSON: artifactsJSON,
		ReceiptJSON:   receiptJSON,
	})
	return err
}

func (s *JudgeService) buildReceipt(result JudgeResult) string {
	receipt, err := BuildReceipt(receiptInput{SubmissionID: result.SubmissionID, ResultJSON: mustMarshalJSON(result)})
	if err != nil {
		return ""
	}

	return mustMarshalJSON(receipt)
}

func (s *JudgeService) failSubmission(submission *stores.SubmissionModel, cause error) error {
	s.logError("judge failure", cause)
	if submission == nil {
		return cause
	}

	result := buildInternalErrorResult(submission.ID)
	_ = s.persistJudgeResult(result, submission)

	finishedAt := time.Now().UTC()
	updateInput := stores.SubmissionStatusUpdateInput{
		SubmissionID: submission.ID,
		Status:       stores.SubmissionStatusFailed,
		FinishedAt:   &finishedAt,
	}
	if err := s.Store.UpdateSubmissionStatus(updateInput); err != nil {
		return err
	}

	return cause
}

func buildInternalErrorResult(submissionID uint) JudgeResult {
	overall := JudgeOverall{Verdict: stores.VerdictInternalError}
	compile := JudgeCompile{Verdict: stores.VerdictInternalError}

	return JudgeResult{
		SubmissionID: submissionID,
		Overall:      overall,
		Compile:      compile,
		Tests:        []JudgeTestResult{},
		Artifacts:    JudgeArtifacts{},
		Timing:       JudgeTiming{},
	}
}

func (s *JudgeService) resolveRunner() Runner {
	if s.Runner != nil {
		return s.Runner
	}

	return &DockerRunner{}
}

func (s *JudgeService) logError(message string, err error) {
	if s.Logger == nil {
		return
	}

	s.Logger.Printf("error [judge]: %s: %v", message, err)
}
