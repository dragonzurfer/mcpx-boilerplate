package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

const (
	AnalysisStatusCompleted = "COMPLETED"
	AnalysisStatusFailed    = "FAILED"
)

type AIAnalysisService struct {
	Store  *stores.Store
	AI     *GeminiClient
	Logger Logger
}

type AnalysisPolicy struct {
	Mode              string `json:"analysis_mode"`
	HintLevel         int    `json:"hint_level"`
	AllowFullSolution bool   `json:"allow_full_solution"`
}

type AnalysisResponse struct {
	AnalysisID string         `json:"analysis_id"`
	Status     string         `json:"status"`
	Result     AnalysisResult `json:"result"`
}

type AnalysisResult struct {
	Summary            string              `json:"summary"`
	RootCauses         []AnalysisRootCause `json:"root_causes"`
	Hints              []AnalysisHint      `json:"hints"`
	ComplexityFeedback ComplexityFeedback  `json:"complexity_feedback"`
	NextActions        []string            `json:"next_actions"`
}

type AnalysisRootCause struct {
	Type       string    `json:"type"`
	Confidence float64   `json:"confidence"`
	CodeRefs   []CodeRef `json:"code_refs"`
}

type CodeRef struct {
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
}

type AnalysisHint struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

type ComplexityFeedback struct {
	Detected ComplexityEstimate `json:"detected"`
	Target   ComplexityEstimate `json:"target"`
}

type ComplexityEstimate struct {
	Time  string `json:"time"`
	Space string `json:"space"`
}

type AnalysisSubmissionInput struct {
	SubmissionID uint
	UserID       uint
	Policy       AnalysisPolicy
}

type analysisContext struct {
	Submission *stores.SubmissionModel
	Result     *stores.SubmissionResultModel
	Problem    *stores.ProblemModel
	Policy     AnalysisPolicy
}

type analysisFingerprint struct {
	SubmissionID uint
	CodeHash     string
	ResultHash   string
	PolicyJSON   string
}

func (s *AIAnalysisService) AnalyzeSubmission(input AnalysisSubmissionInput) (AnalysisResponse, error) {
	context, err := s.loadAnalysisContext(input)
	if err != nil {
		return AnalysisResponse{}, err
	}

	fingerprint := buildAnalysisFingerprint(context)

	cached, err := s.findCachedAnalysis(fingerprint)
	if err == nil {
		return buildAnalysisResponse(cached), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AnalysisResponse{}, err
	}

	analysisID := GenerateAnalysisID()

	analysisModel, err := s.createAnalysisRecord(analysisCreateInput{
		AnalysisID:   analysisID,
		SubmissionID: context.Submission.ID,
		UserID:       context.Submission.UserID,
		CodeHash:     fingerprint.CodeHash,
		ResultHash:   fingerprint.ResultHash,
		PolicyJSON:   fingerprint.PolicyJSON,
	})
	if err != nil {
		return AnalysisResponse{}, err
	}

	analysisResult := s.generateAnalysisResult(analysisGenerateInput(context))

	s.completeAnalysisRecord(analysisCompleteInput{
		AnalysisID: analysisModel.AnalysisID,
		Result:     analysisResult,
	})

	return AnalysisResponse{AnalysisID: analysisModel.AnalysisID, Status: AnalysisStatusCompleted, Result: analysisResult}, nil
}

type AnalysisVerifyInput struct {
	SubmissionID uint
	UserID       uint
	Language     string
	Code         string
	JudgeResult  JudgeResult
	Receipt      Receipt
	Policy       AnalysisPolicy
}

type analysisCreateInput struct {
	AnalysisID   string
	SubmissionID uint
	UserID       uint
	CodeHash     string
	ResultHash   string
	PolicyJSON   string
}

type analysisCompleteInput struct {
	AnalysisID string
	Result     AnalysisResult
}

func (s *AIAnalysisService) AnalyzeVerifiedPayload(input AnalysisVerifyInput) (AnalysisResponse, error) {
	resultJSON := mustMarshalJSON(input.JudgeResult)
	if err := VerifyReceipt(receiptVerifyInput{Receipt: input.Receipt, ResultJSON: resultJSON}); err != nil {
		return AnalysisResponse{}, err
	}
	if input.Receipt.SubmissionID != 0 && input.Receipt.SubmissionID != input.SubmissionID {
		return AnalysisResponse{}, errors.New("receipt submission mismatch")
	}

	policyJSON := mustMarshalJSON(input.Policy)
	codeHash := HashString(input.Code)
	resultHash := HashString(resultJSON)
	analysisID := GenerateAnalysisID()

	analysisModel, err := s.createAnalysisRecord(analysisCreateInput{
		AnalysisID:   analysisID,
		SubmissionID: input.SubmissionID,
		UserID:       input.UserID,
		CodeHash:     codeHash,
		ResultHash:   resultHash,
		PolicyJSON:   policyJSON,
	})
	if err != nil {
		return AnalysisResponse{}, err
	}

	resultPayload := parseJSON(resultJSON)
	analysisResult := s.fallbackAnalysisResult(fallbackAnalysisInput{Result: resultPayload, Policy: input.Policy})

	s.completeAnalysisRecord(analysisCompleteInput{
		AnalysisID: analysisModel.AnalysisID,
		Result:     analysisResult,
	})

	return AnalysisResponse{AnalysisID: analysisModel.AnalysisID, Status: AnalysisStatusCompleted, Result: analysisResult}, nil
}

func (s *AIAnalysisService) loadAnalysisContext(input AnalysisSubmissionInput) (analysisContext, error) {
	submission, err := s.Store.GetSubmissionByID(stores.SubmissionLookupInput{SubmissionID: input.SubmissionID})
	if err != nil {
		return analysisContext{}, err
	}

	if submission.UserID != input.UserID {
		return analysisContext{}, errors.New("forbidden")
	}

	resultModel, err := s.Store.GetSubmissionResult(stores.SubmissionResultLookupInput{SubmissionID: submission.ID})
	if err != nil {
		return analysisContext{}, err
	}

	problemModel, err := s.Store.GetProblemByID(stores.ProblemByIDLookupInput{ProblemID: submission.ProblemID})
	if err != nil {
		return analysisContext{}, err
	}

	return analysisContext{
		Submission: submission,
		Result:     resultModel,
		Problem:    problemModel,
		Policy:     input.Policy,
	}, nil
}

func buildAnalysisFingerprint(context analysisContext) analysisFingerprint {
	policyJSON := mustMarshalJSON(context.Policy)
	codeHash := HashString(context.Submission.CodeText)
	resultHash := HashString(buildResultFingerprint(context.Result))

	return analysisFingerprint{
		SubmissionID: context.Submission.ID,
		CodeHash:     codeHash,
		ResultHash:   resultHash,
		PolicyJSON:   policyJSON,
	}
}

func (s *AIAnalysisService) findCachedAnalysis(fingerprint analysisFingerprint) (*stores.AIAnalysisModel, error) {
	return s.Store.FindAIAnalysisByFingerprint(stores.AIAnalysisFingerprintInput{
		SubmissionID: fingerprint.SubmissionID,
		CodeHash:     fingerprint.CodeHash,
		ResultHash:   fingerprint.ResultHash,
		PolicyJSON:   fingerprint.PolicyJSON,
	})
}

func (s *AIAnalysisService) createAnalysisRecord(input analysisCreateInput) (*stores.AIAnalysisModel, error) {
	return s.Store.CreateAIAnalysis(stores.AIAnalysisCreateInput{
		AnalysisID:   input.AnalysisID,
		SubmissionID: input.SubmissionID,
		UserID:       input.UserID,
		CodeHash:     input.CodeHash,
		ResultHash:   input.ResultHash,
		PolicyJSON:   input.PolicyJSON,
		Status:       AnalysisStatusFailed,
	})
}

func (s *AIAnalysisService) completeAnalysisRecord(input analysisCompleteInput) {
	responseJSON := mustMarshalJSON(input.Result)
	_, _ = s.Store.UpdateAIAnalysis(stores.AIAnalysisUpdateInput{
		AnalysisID:   input.AnalysisID,
		Status:       AnalysisStatusCompleted,
		ResponseJSON: responseJSON,
	})
}

type analysisGenerateInput struct {
	Submission *stores.SubmissionModel
	Result     *stores.SubmissionResultModel
	Problem    *stores.ProblemModel
	Policy     AnalysisPolicy
}

func (s *AIAnalysisService) generateAnalysisResult(input analysisGenerateInput) AnalysisResult {
	prompt := buildPracticePrompt(practicePromptInput(input))
	if s.AI == nil {
		return s.fallbackAnalysisResult(fallbackAnalysisInput{Result: buildResultPayload(input.Result), Policy: input.Policy})
	}

	output, err := s.AI.GeneratePracticeAnalysis(context.Background(), PracticeAnalysisInput{Prompt: prompt})
	if err != nil {
		s.logError("ai analysis failed", err)
		return s.fallbackAnalysisResult(fallbackAnalysisInput{Result: buildResultPayload(input.Result), Policy: input.Policy})
	}

	return output
}

type fallbackAnalysisInput struct {
	Result map[string]interface{}
	Policy AnalysisPolicy
}

func (s *AIAnalysisService) fallbackAnalysisResult(input fallbackAnalysisInput) AnalysisResult {
	verdict := extractVerdict(input.Result)

	summary := fallbackSummary(verdict)
	rootCause := fallbackRootCause(verdict)
	hints := fallbackHints(verdict, input.Policy.HintLevel)
	nextActions := fallbackActions(verdict)

	return AnalysisResult{
		Summary:    summary,
		RootCauses: rootCause,
		Hints:      hints,
		ComplexityFeedback: ComplexityFeedback{
			Detected: ComplexityEstimate{},
			Target:   ComplexityEstimate{},
		},
		NextActions: nextActions,
	}
}

func buildAnalysisResponse(model *stores.AIAnalysisModel) AnalysisResponse {
	result := AnalysisResult{}
	_ = json.Unmarshal([]byte(model.ResponseJSON), &result)

	status := strings.TrimSpace(model.Status)
	if status == "" {
		status = AnalysisStatusCompleted
	}

	return AnalysisResponse{AnalysisID: model.AnalysisID, Status: status, Result: result}
}

func buildResultPayload(result *stores.SubmissionResultModel) map[string]interface{} {
	return map[string]interface{}{
		"overall":   parseJSON(result.OverallJSON),
		"compile":   parseJSON(result.CompileJSON),
		"tests":     parseJSONArrayValue(result.TestsJSON),
		"timing":    parseJSON(result.TimingJSON),
		"artifacts": parseJSON(result.ArtifactsJSON),
	}
}

func buildResultFingerprint(result *stores.SubmissionResultModel) string {
	if result == nil {
		return ""
	}

	parts := []string{
		strings.TrimSpace(result.OverallJSON),
		strings.TrimSpace(result.CompileJSON),
		strings.TrimSpace(result.TestsJSON),
		strings.TrimSpace(result.TimingJSON),
		strings.TrimSpace(result.ArtifactsJSON),
	}

	return strings.Join(parts, "|")
}

func buildPracticePrompt(input practicePromptInput) string {
	payload := map[string]interface{}{
		"policy":     input.Policy,
		"problem":    problemContext(input.Problem),
		"submission": submissionContext(input.Submission),
		"result":     buildResultPayload(input.Result),
	}

	encoded, _ := json.Marshal(payload)

	return practicePromptPrefix + string(encoded)
}

type practicePromptInput struct {
	Submission *stores.SubmissionModel
	Result     *stores.SubmissionResultModel
	Problem    *stores.ProblemModel
	Policy     AnalysisPolicy
}

func problemContext(problem *stores.ProblemModel) map[string]interface{} {
	if problem == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"title":       problem.Title,
		"statement":   stores.ParseProblemStatement(problem.StatementJSON),
		"constraints": parseJSON(problem.ConstraintsJSON),
		"io_spec":     parseJSON(problem.IOSpecJSON),
		"tags":        stores.ParseProblemTags(problem.TagsJSON),
	}
}

func submissionContext(submission *stores.SubmissionModel) map[string]interface{} {
	if submission == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"language": submission.Language,
		"mode":     submission.Mode,
		"code":     submission.CodeText,
	}
}

func extractVerdict(result map[string]interface{}) string {
	overall, ok := result["overall"].(map[string]interface{})
	if !ok {
		return ""
	}

	verdict, _ := overall["verdict"].(string)
	return verdict
}

func fallbackSummary(verdict string) string {
	switch verdict {
	case stores.VerdictCompileError:
		return "Compilation failed. Fix the errors and try again."
	case stores.VerdictWrongAnswer:
		return "Your solution produced a wrong answer on at least one testcase."
	case stores.VerdictTLE:
		return "Time limit exceeded. Your solution needs to be faster."
	case stores.VerdictMLE:
		return "Memory limit exceeded. Reduce memory usage."
	case stores.VerdictRuntimeError:
		return "Runtime error. Check for crashes or invalid memory access."
	case stores.VerdictOutputLimitExceeded:
		return "Output limit exceeded. Reduce output size or remove debug logs."
	case stores.VerdictAccepted:
		return "Accepted. Great job!"
	default:
		return "We could not analyze this submission yet."
	}
}

func fallbackRootCause(verdict string) []AnalysisRootCause {
	if verdict == stores.VerdictAccepted {
		return []AnalysisRootCause{}
	}

	causeType := "LOGIC_BUG"
	switch verdict {
	case stores.VerdictCompileError:
		causeType = "COMPILE_ERROR"
	case stores.VerdictTLE:
		causeType = "INEFFICIENT"
	case stores.VerdictMLE:
		causeType = "MEMORY"
	case stores.VerdictRuntimeError:
		causeType = "RUNTIME_ERROR"
	case stores.VerdictOutputLimitExceeded:
		causeType = "OUTPUT_LIMIT"
	}

	return []AnalysisRootCause{{Type: causeType, Confidence: 0.6}}
}

func fallbackHints(verdict string, hintLevel int) []AnalysisHint {
	if hintLevel <= 0 {
		hintLevel = 1
	}

	hints := []AnalysisHint{}
	if verdict == stores.VerdictCompileError {
		return []AnalysisHint{{Level: hintLevel, Text: "Review compiler errors and fix syntax or missing imports."}}
	}
	if verdict == stores.VerdictTLE {
		return []AnalysisHint{{Level: hintLevel, Text: "Look for a more efficient algorithm or avoid nested loops."}}
	}
	if verdict == stores.VerdictWrongAnswer {
		return []AnalysisHint{{Level: hintLevel, Text: "Check edge cases and verify your output format."}}
	}

	hints = append(hints, AnalysisHint{Level: hintLevel, Text: "Review the problem constraints and test with small edge cases."})
	return hints
}

func fallbackActions(verdict string) []string {
	if verdict == stores.VerdictAccepted {
		return []string{"Try the next problem to keep momentum."}
	}

	return []string{"Run with custom tests to isolate the failing case.", "Refactor the solution and re-submit."}
}

func parseJSON(raw string) map[string]interface{} {
	result := map[string]interface{}{}
	if strings.TrimSpace(raw) == "" {
		return result
	}

	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return map[string]interface{}{}
	}

	return result
}

func parseJSONArrayValue(raw string) []interface{} {
	items := []interface{}{}
	if strings.TrimSpace(raw) == "" {
		return items
	}

	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []interface{}{}
	}

	return items
}

func (s *AIAnalysisService) logError(message string, err error) {
	if s.Logger == nil {
		return
	}

	s.Logger.Printf("error [ai-analysis]: %s: %v", message, err)
}

const practicePromptPrefix = `You are a coding coach. Respond ONLY with JSON matching this schema:
{
  "summary": "string",
  "root_causes": [{"type":"LOGIC_BUG","confidence":0.5,"code_refs":[{"start_line":1,"end_line":1}]}],
  "hints": [{"level":1,"text":"string"}],
  "complexity_feedback": {"detected":{"time":"O(n)","space":"O(n)"},"target":{"time":"O(n)","space":"O(n)"}},
  "next_actions": ["string"]
}
Use the provided payload to reason about the bug. Keep hints concise. Do not leak hidden tests.
Payload:`
