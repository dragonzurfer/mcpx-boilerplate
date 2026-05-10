package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

const (
	AnalysisStatusCompleted = "COMPLETED"
	AnalysisStatusFailed    = "FAILED"
)

const (
	maxPromptStatementRunes  = 7000
	maxPromptEditorialRunes  = 7000
	maxPromptCodeRunes       = 12000
	maxPromptNumberedRunes   = 14000
	maxPromptSolutionRunes   = 2400
	maxPromptActionRunes     = 200
	maxPromptOfficialEntries = 3
	maxPromptReferenceRows   = 3
	maxPromptHints           = 8
	maxPromptTestsSample     = 5
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
	Submission         *stores.SubmissionModel
	Result             *stores.SubmissionResultModel
	Problem            *stores.ProblemModel
	ReferenceSolutions []stores.SolutionModel
	Policy             AnalysisPolicy
}

type analysisFingerprint struct {
	SubmissionID uint
	CodeHash     string
	ResultHash   string
	PolicyJSON   string
}

func (s *AIAnalysisService) AnalyzeSubmission(input AnalysisSubmissionInput) (AnalysisResponse, error) {
	analysisContextValue, err := s.loadAnalysisContext(input)
	if err != nil {
		return AnalysisResponse{}, err
	}

	fingerprint := buildAnalysisFingerprint(analysisContextValue)
	cached, err := s.findCachedAnalysis(fingerprint)
	if err == nil {
		return buildAnalysisResponse(cached), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AnalysisResponse{}, err
	}

	analysisModel, err := s.createAnalysisRecord(buildCreateInput(createRecordSource{
		AnalysisID:   GenerateAnalysisID(),
		SubmissionID: analysisContextValue.Submission.ID,
		UserID:       analysisContextValue.Submission.UserID,
		CodeHash:     fingerprint.CodeHash,
		ResultHash:   fingerprint.ResultHash,
		PolicyJSON:   fingerprint.PolicyJSON,
	}))
	if err != nil {
		return AnalysisResponse{}, err
	}

	analysisResult := s.generateAnalysisResult(buildGenerateInputFromContext(analysisContextValue))
	s.completeAnalysisRecord(analysisCompleteInput{AnalysisID: analysisModel.AnalysisID, Result: analysisResult})

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

	submissionModel, err := s.loadOwnedSubmission(ownedSubmissionInput{SubmissionID: input.SubmissionID, UserID: input.UserID})
	if err != nil {
		return AnalysisResponse{}, err
	}

	problemLoadOutput, err := s.loadProblemContext(problemContextLoadInput{ProblemID: submissionModel.ProblemID})
	if err != nil {
		return AnalysisResponse{}, err
	}

	problemFingerprint := buildProblemFingerprint(problemFingerprintInput(problemLoadOutput))

	analysisModel, err := s.createAnalysisRecord(buildCreateInput(createRecordSource{
		AnalysisID:   GenerateAnalysisID(),
		SubmissionID: input.SubmissionID,
		UserID:       input.UserID,
		CodeHash:     HashString(strings.TrimSpace(input.Code)),
		ResultHash:   HashString(resultJSON + "|" + problemFingerprint),
		PolicyJSON:   mustMarshalJSON(input.Policy),
	}))
	if err != nil {
		return AnalysisResponse{}, err
	}

	analysisLanguage := resolveAnalysisLanguage(resolveLanguageInput{
		RequestedLanguage: input.Language,
		StoredLanguage:    submissionModel.Language,
	})

	analysisResult := s.generateAnalysisResult(analysisGenerateInput{
		SubmissionID:       input.SubmissionID,
		Language:           analysisLanguage,
		SubmissionMode:     submissionModel.Mode,
		SourceCode:         strings.TrimSpace(input.Code),
		ResultPayload:      parseJSON(resultJSON),
		Problem:            problemLoadOutput.Problem,
		ReferenceSolutions: problemLoadOutput.ReferenceSolutions,
		Policy:             input.Policy,
	})

	s.completeAnalysisRecord(analysisCompleteInput{AnalysisID: analysisModel.AnalysisID, Result: analysisResult})
	return AnalysisResponse{AnalysisID: analysisModel.AnalysisID, Status: AnalysisStatusCompleted, Result: analysisResult}, nil
}

type ownedSubmissionInput struct {
	SubmissionID uint
	UserID       uint
}

func (s *AIAnalysisService) loadOwnedSubmission(input ownedSubmissionInput) (*stores.SubmissionModel, error) {
	submissionModel, err := s.Store.GetSubmissionByID(stores.SubmissionLookupInput{SubmissionID: input.SubmissionID})
	if err != nil {
		return nil, err
	}

	if submissionModel.UserID != input.UserID {
		return nil, errors.New("forbidden")
	}

	return submissionModel, nil
}

type problemContextLoadInput struct {
	ProblemID uint
}

type problemContextLoadOutput struct {
	Problem            *stores.ProblemModel
	ReferenceSolutions []stores.SolutionModel
}

func (s *AIAnalysisService) loadProblemContext(input problemContextLoadInput) (problemContextLoadOutput, error) {
	problemModel, err := s.Store.GetProblemByID(stores.ProblemByIDLookupInput{ProblemID: input.ProblemID})
	if err != nil {
		return problemContextLoadOutput{}, err
	}

	solutionRows, err := s.Store.ListSolutions(stores.SolutionListInput{ProblemID: input.ProblemID})
	if err != nil {
		s.logError("list reference solutions failed", err)
		return problemContextLoadOutput{Problem: problemModel, ReferenceSolutions: []stores.SolutionModel{}}, nil
	}

	referenceSolutions := selectReferenceSolutions(solutionRows.Solutions)
	return problemContextLoadOutput{Problem: problemModel, ReferenceSolutions: referenceSolutions}, nil
}

func selectReferenceSolutions(solutionRows []stores.SolutionModel) []stores.SolutionModel {
	if len(solutionRows) == 0 {
		return []stores.SolutionModel{}
	}

	referenceSolutions := make([]stores.SolutionModel, 0, len(solutionRows))
	for _, solutionRow := range solutionRows {
		if !solutionRow.IsReference {
			continue
		}
		referenceSolutions = append(referenceSolutions, solutionRow)
	}

	return referenceSolutions
}

func (s *AIAnalysisService) loadAnalysisContext(input AnalysisSubmissionInput) (analysisContext, error) {
	submissionModel, err := s.loadOwnedSubmission(ownedSubmissionInput{
		SubmissionID: input.SubmissionID,
		UserID:       input.UserID,
	})
	if err != nil {
		return analysisContext{}, err
	}

	resultModel, err := s.Store.GetSubmissionResult(stores.SubmissionResultLookupInput{SubmissionID: submissionModel.ID})
	if err != nil {
		return analysisContext{}, err
	}

	problemLoadOutput, err := s.loadProblemContext(problemContextLoadInput{ProblemID: submissionModel.ProblemID})
	if err != nil {
		return analysisContext{}, err
	}

	return analysisContext{
		Submission:         submissionModel,
		Result:             resultModel,
		Problem:            problemLoadOutput.Problem,
		ReferenceSolutions: problemLoadOutput.ReferenceSolutions,
		Policy:             input.Policy,
	}, nil
}

func buildAnalysisFingerprint(analysisContextValue analysisContext) analysisFingerprint {
	policyJSON := mustMarshalJSON(analysisContextValue.Policy)
	codeHash := HashString(strings.TrimSpace(analysisContextValue.Submission.CodeText))

	resultFingerprint := buildResultFingerprint(analysisContextValue.Result)
	problemFingerprint := buildProblemFingerprint(problemFingerprintInput{
		Problem:            analysisContextValue.Problem,
		ReferenceSolutions: analysisContextValue.ReferenceSolutions,
	})
	resultHash := HashString(resultFingerprint + "|" + problemFingerprint)

	return analysisFingerprint{
		SubmissionID: analysisContextValue.Submission.ID,
		CodeHash:     codeHash,
		ResultHash:   resultHash,
		PolicyJSON:   policyJSON,
	}
}

type problemFingerprintInput struct {
	Problem            *stores.ProblemModel
	ReferenceSolutions []stores.SolutionModel
}

func buildProblemFingerprint(input problemFingerprintInput) string {
	if input.Problem == nil {
		return ""
	}

	parts := []string{
		strings.TrimSpace(input.Problem.Title),
		strings.TrimSpace(input.Problem.StatementJSON),
		strings.TrimSpace(input.Problem.IOSpecJSON),
		strings.TrimSpace(input.Problem.ConstraintsJSON),
		strings.TrimSpace(input.Problem.EditorialJSON),
		strings.TrimSpace(input.Problem.SolutionsJSON),
	}

	for _, referenceSolution := range input.ReferenceSolutions {
		referenceFingerprint := strings.Join([]string{
			strings.TrimSpace(referenceSolution.Language),
			strings.TrimSpace(referenceSolution.ApproachSummary),
			strings.TrimSpace(referenceSolution.ComplexityJSON),
			strings.TrimSpace(referenceSolution.CodeText),
		}, "|")
		parts = append(parts, referenceFingerprint)
	}

	return strings.Join(parts, "||")
}

func (s *AIAnalysisService) findCachedAnalysis(fingerprint analysisFingerprint) (*stores.AIAnalysisModel, error) {
	return s.Store.FindAIAnalysisByFingerprint(stores.AIAnalysisFingerprintInput{
		SubmissionID: fingerprint.SubmissionID,
		CodeHash:     fingerprint.CodeHash,
		ResultHash:   fingerprint.ResultHash,
		PolicyJSON:   fingerprint.PolicyJSON,
	})
}

type createRecordSource struct {
	AnalysisID   string
	SubmissionID uint
	UserID       uint
	CodeHash     string
	ResultHash   string
	PolicyJSON   string
}

func buildCreateInput(input createRecordSource) analysisCreateInput {
	return analysisCreateInput(input)
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
	SubmissionID       uint
	Language           string
	SubmissionMode     string
	SourceCode         string
	ResultPayload      map[string]interface{}
	Problem            *stores.ProblemModel
	ReferenceSolutions []stores.SolutionModel
	Policy             AnalysisPolicy
}

func buildGenerateInputFromContext(input analysisContext) analysisGenerateInput {
	return analysisGenerateInput{
		SubmissionID:       input.Submission.ID,
		Language:           input.Submission.Language,
		SubmissionMode:     input.Submission.Mode,
		SourceCode:         input.Submission.CodeText,
		ResultPayload:      buildResultPayload(input.Result),
		Problem:            input.Problem,
		ReferenceSolutions: input.ReferenceSolutions,
		Policy:             input.Policy,
	}
}

func (s *AIAnalysisService) generateAnalysisResult(input analysisGenerateInput) AnalysisResult {
	resultPayload := normalizeResultPayload(input.ResultPayload)
	promptInput := buildPracticePromptInput(input)
	prompt := buildPracticePrompt(promptInput)

	if s.AI == nil {
		return s.fallbackAnalysisResult(fallbackAnalysisInput{Result: resultPayload, Policy: input.Policy})
	}

	analysisOutput, err := s.AI.GeneratePracticeAnalysis(context.Background(), PracticeAnalysisInput{Prompt: prompt})
	if err != nil {
		s.logError("ai analysis failed", err)
		return s.fallbackAnalysisResult(fallbackAnalysisInput{Result: resultPayload, Policy: input.Policy})
	}

	return normalizeAnalysisOutput(normalizeAnalysisOutputInput{Result: analysisOutput, HintLevel: input.Policy.HintLevel})
}

func buildPracticePromptInput(input analysisGenerateInput) practicePromptInput {
	submissionCode := strings.TrimSpace(input.SourceCode)

	return practicePromptInput{
		Policy:             input.Policy,
		Problem:            input.Problem,
		ReferenceSolutions: input.ReferenceSolutions,
		ResultPayload:      normalizeResultPayload(input.ResultPayload),
		Submission: promptSubmissionInput{
			Language: strings.TrimSpace(input.Language),
			Mode:     strings.TrimSpace(input.SubmissionMode),
			Code:     submissionCode,
		},
	}
}

type normalizeAnalysisOutputInput struct {
	Result    AnalysisResult
	HintLevel int
}

func normalizeAnalysisOutput(input normalizeAnalysisOutputInput) AnalysisResult {
	normalized := input.Result
	normalized.Summary = strings.TrimSpace(normalized.Summary)
	normalized.RootCauses = ensureRootCauseSlice(normalized.RootCauses)
	normalized.Hints = ensureHintSlice(ensureHintLevelInput{Hints: normalized.Hints, HintLevel: input.HintLevel})
	normalized.NextActions = ensureActionSlice(normalized.NextActions)
	return normalized
}

func ensureRootCauseSlice(causes []AnalysisRootCause) []AnalysisRootCause {
	if causes == nil {
		return []AnalysisRootCause{}
	}
	return causes
}

type ensureHintLevelInput struct {
	Hints     []AnalysisHint
	HintLevel int
}

func ensureHintSlice(input ensureHintLevelInput) []AnalysisHint {
	if input.Hints == nil {
		return []AnalysisHint{}
	}

	resolvedLevel := input.HintLevel
	if resolvedLevel <= 0 {
		resolvedLevel = 1
	}

	normalized := make([]AnalysisHint, 0, len(input.Hints))
	for _, hint := range input.Hints {
		hintText := strings.TrimSpace(hint.Text)
		if hintText == "" {
			continue
		}
		if hint.Level <= 0 {
			hint.Level = resolvedLevel
		}
		normalized = append(normalized, AnalysisHint{Level: hint.Level, Text: trimText(hintText, 240)})
	}

	return normalized
}

func ensureActionSlice(actions []string) []string {
	if actions == nil {
		return []string{}
	}

	normalized := make([]string, 0, len(actions))
	for _, action := range actions {
		actionText := strings.TrimSpace(action)
		if actionText == "" {
			continue
		}
		normalized = append(normalized, trimText(actionText, maxPromptActionRunes))
	}

	return normalized
}

type resolveLanguageInput struct {
	RequestedLanguage string
	StoredLanguage    string
}

func resolveAnalysisLanguage(input resolveLanguageInput) string {
	requestedLanguage := strings.TrimSpace(input.RequestedLanguage)
	if requestedLanguage != "" {
		return requestedLanguage
	}

	return strings.TrimSpace(input.StoredLanguage)
}

type fallbackAnalysisInput struct {
	Result map[string]interface{}
	Policy AnalysisPolicy
}

func (s *AIAnalysisService) fallbackAnalysisResult(input fallbackAnalysisInput) AnalysisResult {
	verdict := extractVerdict(input.Result)
	firstFailingTest := extractFirstFailingTestcase(extractTestsFromPayload(input.Result))

	summary := fallbackSummary(fallbackSummaryInput{Verdict: verdict, FirstFailingTest: firstFailingTest})
	rootCause := fallbackRootCause(verdict)
	hints := fallbackHints(fallbackHintsInput{
		Verdict:          verdict,
		HintLevel:        input.Policy.HintLevel,
		FirstFailingTest: firstFailingTest,
	})
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

type fallbackSummaryInput struct {
	Verdict          string
	FirstFailingTest *promptFailingTestcase
}

func fallbackSummary(input fallbackSummaryInput) string {
	switch input.Verdict {
	case stores.VerdictCompileError:
		return "Compilation failed. Fix the compiler errors and submit again."
	case stores.VerdictWrongAnswer:
		if input.FirstFailingTest != nil {
			actualOutput := strings.TrimSpace(input.FirstFailingTest.Detail.Actual)
			expectedOutput := strings.TrimSpace(input.FirstFailingTest.Detail.Expected)
			if actualOutput != "" && expectedOutput != "" {
				return "Wrong answer on the first failing testcase: expected output differs from your program output."
			}
		}
		return "Your solution produced a wrong answer on at least one testcase."
	case stores.VerdictTLE:
		return "Time limit exceeded. Your current approach is too slow for the input limits."
	case stores.VerdictMLE:
		return "Memory limit exceeded. Reduce memory usage and data-structure overhead."
	case stores.VerdictRuntimeError:
		return "Runtime error. Check for invalid indexing, division by zero, and unhandled edge cases."
	case stores.VerdictOutputLimitExceeded:
		return "Output limit exceeded. Remove debug prints and keep output strictly to the expected format."
	case stores.VerdictAccepted:
		return "Accepted. Your submission passed this dataset."
	default:
		return "We could not analyze this submission yet."
	}
}

func fallbackRootCause(verdict string) []AnalysisRootCause {
	if verdict == stores.VerdictAccepted {
		return []AnalysisRootCause{}
	}

	rootCauseType := "LOGIC_BUG"
	switch verdict {
	case stores.VerdictCompileError:
		rootCauseType = "COMPILE_ERROR"
	case stores.VerdictTLE:
		rootCauseType = "INEFFICIENT"
	case stores.VerdictMLE:
		rootCauseType = "MEMORY"
	case stores.VerdictRuntimeError:
		rootCauseType = "RUNTIME_ERROR"
	case stores.VerdictOutputLimitExceeded:
		rootCauseType = "OUTPUT_LIMIT"
	}

	return []AnalysisRootCause{{Type: rootCauseType, Confidence: 0.6}}
}

type fallbackHintsInput struct {
	Verdict          string
	HintLevel        int
	FirstFailingTest *promptFailingTestcase
}

func fallbackHints(input fallbackHintsInput) []AnalysisHint {
	hintLevel := input.HintLevel
	if hintLevel <= 0 {
		hintLevel = 1
	}

	switch input.Verdict {
	case stores.VerdictCompileError:
		return []AnalysisHint{{Level: hintLevel, Text: "Read the compiler error top-down and fix the first reported error before the rest."}}
	case stores.VerdictTLE:
		return []AnalysisHint{{Level: hintLevel, Text: "Re-check the complexity against constraints and avoid repeated work inside nested loops."}}
	case stores.VerdictWrongAnswer:
		if input.FirstFailingTest != nil {
			return []AnalysisHint{{Level: hintLevel, Text: "Replay the first failing testcase manually and verify each state transition in your current logic."}}
		}
		return []AnalysisHint{{Level: hintLevel, Text: "Check edge cases and confirm output formatting exactly matches the expected format."}}
	default:
		return []AnalysisHint{{Level: hintLevel, Text: "Re-check constraints, edge cases, and run targeted custom tests before resubmitting."}}
	}
}

func fallbackActions(verdict string) []string {
	if verdict == stores.VerdictAccepted {
		return []string{"Try a harder variant or optimize for cleaner complexity."}
	}

	return []string{
		"Use the first failing testcase to debug your current logic step-by-step.",
		"Revise the algorithm and submit again after local dry-run checks.",
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
	if result == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"overall":   parseJSON(result.OverallJSON),
		"compile":   parseJSON(result.CompileJSON),
		"tests":     parseJSONArrayValue(result.TestsJSON),
		"timing":    parseJSON(result.TimingJSON),
		"artifacts": parseJSON(result.ArtifactsJSON),
	}
}

func normalizeResultPayload(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"overall":   mapFromInterface(payload["overall"]),
		"compile":   mapFromInterface(payload["compile"]),
		"tests":     arrayFromInterface(payload["tests"]),
		"timing":    mapFromInterface(payload["timing"]),
		"artifacts": mapFromInterface(payload["artifacts"]),
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

type practicePromptInput struct {
	Policy             AnalysisPolicy
	Problem            *stores.ProblemModel
	ReferenceSolutions []stores.SolutionModel
	Submission         promptSubmissionInput
	ResultPayload      map[string]interface{}
}

type promptSubmissionInput struct {
	Language string
	Mode     string
	Code     string
}

func buildPracticePrompt(input practicePromptInput) string {
	promptPayload := buildPromptPayload(input)
	encodedPayload := marshalPromptJSON(promptPayload)
	return practicePromptPrefix + encodedPayload
}

type practicePromptPayload struct {
	Policy           AnalysisPolicy          `json:"policy"`
	CoachingWorkflow []string                `json:"coaching_workflow"`
	Guardrails       []string                `json:"guardrails"`
	Problem          promptProblemContext    `json:"problem"`
	Submission       promptSubmissionContext `json:"submission"`
	Result           promptResultContext     `json:"result"`
}

func buildPromptPayload(input practicePromptInput) practicePromptPayload {
	return practicePromptPayload{
		Policy: input.Policy,
		CoachingWorkflow: []string{
			"Infer the intended algorithm from the submission code before diagnosing failures.",
			"Cross-check the intended algorithm with the problem editorial and reference approaches.",
			"Use compile/test evidence to identify concrete root causes and code-level mistakes.",
			"Provide hints that guide the learner to the fix, not random generic advice.",
		},
		Guardrails: []string{
			"Never reveal hidden testcase contents that are not in payload.",
			"Respect policy.allow_full_solution: do not give full code when false.",
			"Keep hints concise and progressive by hint_level.",
		},
		Problem: buildPromptProblemContext(promptProblemContextInput{
			Problem:            input.Problem,
			ReferenceSolutions: input.ReferenceSolutions,
		}),
		Submission: buildPromptSubmissionContext(input.Submission),
		Result:     buildPromptResultContext(input.ResultPayload),
	}
}

type promptProblemContextInput struct {
	Problem            *stores.ProblemModel
	ReferenceSolutions []stores.SolutionModel
}

type promptProblemContext struct {
	Title              string                    `json:"title"`
	Difficulty         string                    `json:"difficulty"`
	StatementMarkdown  string                    `json:"statement_markdown"`
	Examples           []stores.ProblemExample   `json:"examples"`
	Notes              []string                  `json:"notes"`
	IOSpec             stores.ProblemIOSpec      `json:"io_spec"`
	Constraints        stores.ProblemConstraints `json:"constraints"`
	Tags               []string                  `json:"tags"`
	EditorialMarkdown  string                    `json:"editorial_markdown"`
	EditorialHints     []string                  `json:"editorial_hints"`
	OfficialSolutions  []promptSolutionContext   `json:"official_solutions"`
	ReferenceSolutions []promptSolutionContext   `json:"reference_solutions"`
}

type promptSolutionContext struct {
	Language    string                 `json:"language,omitempty"`
	Approach    string                 `json:"approach,omitempty"`
	Complexity  map[string]interface{} `json:"complexity,omitempty"`
	CodeSnippet string                 `json:"code_snippet,omitempty"`
}

func buildPromptProblemContext(input promptProblemContextInput) promptProblemContext {
	if input.Problem == nil {
		return promptProblemContext{}
	}

	statement := stores.ParseProblemStatement(input.Problem.StatementJSON)
	constraints := stores.ParseProblemConstraints(input.Problem.ConstraintsJSON)
	editorial := stores.ParseProblemEditorial(input.Problem.EditorialJSON)

	statement.Markdown = trimText(statement.Markdown, maxPromptStatementRunes)
	statement.Examples = trimProblemExamples(statement.Examples)
	statement.Notes = trimStringList(trimStringListInput{Values: statement.Notes, MaxItems: 6, MaxRunes: 260})

	constraints.InputConstraintsMD = trimText(constraints.InputConstraintsMD, 1200)
	constraints.OutputConstraintsMD = trimText(constraints.OutputConstraintsMD, 1200)
	editorial.Markdown = trimText(editorial.Markdown, maxPromptEditorialRunes)
	editorial.Hints = trimStringList(trimStringListInput{Values: editorial.Hints, MaxItems: maxPromptHints, MaxRunes: 260})

	return promptProblemContext{
		Title:              strings.TrimSpace(input.Problem.Title),
		Difficulty:         strings.TrimSpace(input.Problem.Difficulty),
		StatementMarkdown:  statement.Markdown,
		Examples:           statement.Examples,
		Notes:              statement.Notes,
		IOSpec:             stores.ParseProblemIOSpec(input.Problem.IOSpecJSON),
		Constraints:        constraints,
		Tags:               stores.ParseProblemTags(input.Problem.TagsJSON),
		EditorialMarkdown:  editorial.Markdown,
		EditorialHints:     editorial.Hints,
		OfficialSolutions:  buildOfficialPromptSolutions(input.Problem.SolutionsJSON),
		ReferenceSolutions: buildReferencePromptSolutions(input.ReferenceSolutions),
	}
}

func trimProblemExamples(examples []stores.ProblemExample) []stores.ProblemExample {
	if len(examples) == 0 {
		return []stores.ProblemExample{}
	}

	trimmedExamples := make([]stores.ProblemExample, 0, len(examples))
	for _, example := range examples {
		trimmedExamples = append(trimmedExamples, stores.ProblemExample{
			Input:       trimText(example.Input, 600),
			Output:      trimText(example.Output, 600),
			Explanation: trimText(example.Explanation, 600),
		})
		if len(trimmedExamples) >= 4 {
			break
		}
	}

	return trimmedExamples
}

type trimStringListInput struct {
	Values   []string
	MaxItems int
	MaxRunes int
}

func trimStringList(input trimStringListInput) []string {
	if len(input.Values) == 0 {
		return []string{}
	}

	maxItems := input.MaxItems
	if maxItems <= 0 {
		maxItems = len(input.Values)
	}

	trimmed := make([]string, 0, len(input.Values))
	for _, value := range input.Values {
		text := strings.TrimSpace(value)
		if text == "" {
			continue
		}
		trimmed = append(trimmed, trimText(text, input.MaxRunes))
		if len(trimmed) >= maxItems {
			break
		}
	}

	return trimmed
}

func buildOfficialPromptSolutions(rawSolutionsJSON string) []promptSolutionContext {
	solutionItems := parseJSONArrayValue(rawSolutionsJSON)
	if len(solutionItems) == 0 {
		return []promptSolutionContext{}
	}

	solutions := make([]promptSolutionContext, 0, len(solutionItems))
	for _, solutionItem := range solutionItems {
		solutionValues := mapFromInterface(solutionItem)
		if len(solutionValues) == 0 {
			continue
		}

		complexityValues := mapFromInterface(solutionValues["complexity"])
		if len(complexityValues) == 0 {
			complexityValues = parseJSON(stringFromInterface(solutionValues["complexity_json"]))
		}

		solution := promptSolutionContext{
			Language:    trimText(pickFirstString(pickStringInput{Values: solutionValues, Keys: []string{"language", "lang"}}), 24),
			Approach:    trimText(pickFirstString(pickStringInput{Values: solutionValues, Keys: []string{"approach_summary", "summary", "approach", "explanation"}}), 500),
			Complexity:  complexityValues,
			CodeSnippet: trimText(pickFirstString(pickStringInput{Values: solutionValues, Keys: []string{"code", "code_text", "solution"}}), maxPromptSolutionRunes),
		}

		if isPromptSolutionEmpty(solution) {
			continue
		}

		solutions = append(solutions, solution)
		if len(solutions) >= maxPromptOfficialEntries {
			break
		}
	}

	return solutions
}

func buildReferencePromptSolutions(referenceRows []stores.SolutionModel) []promptSolutionContext {
	if len(referenceRows) == 0 {
		return []promptSolutionContext{}
	}

	solutions := make([]promptSolutionContext, 0, len(referenceRows))
	for _, referenceRow := range referenceRows {
		solution := promptSolutionContext{
			Language:    trimText(strings.TrimSpace(referenceRow.Language), 24),
			Approach:    trimText(strings.TrimSpace(referenceRow.ApproachSummary), 500),
			Complexity:  parseJSON(referenceRow.ComplexityJSON),
			CodeSnippet: trimText(strings.TrimSpace(referenceRow.CodeText), maxPromptSolutionRunes),
		}
		if isPromptSolutionEmpty(solution) {
			continue
		}

		solutions = append(solutions, solution)
		if len(solutions) >= maxPromptReferenceRows {
			break
		}
	}

	return solutions
}

func isPromptSolutionEmpty(solution promptSolutionContext) bool {
	if solution.Language != "" {
		return false
	}
	if solution.Approach != "" {
		return false
	}
	if solution.CodeSnippet != "" {
		return false
	}
	return len(solution.Complexity) == 0
}

type pickStringInput struct {
	Values map[string]interface{}
	Keys   []string
}

func pickFirstString(input pickStringInput) string {
	for _, key := range input.Keys {
		value := strings.TrimSpace(stringFromInterface(input.Values[key]))
		if value != "" {
			return value
		}
	}
	return ""
}

type promptSubmissionContext struct {
	Language         string   `json:"language"`
	Mode             string   `json:"mode"`
	Code             string   `json:"code"`
	NumberedCode     string   `json:"numbered_code"`
	CodeSignals      []string `json:"code_signals"`
	ApparentApproach string   `json:"apparent_approach"`
}

func buildPromptSubmissionContext(input promptSubmissionInput) promptSubmissionContext {
	cleanCode := trimText(strings.TrimSpace(input.Code), maxPromptCodeRunes)
	signals := detectCodeSignals(cleanCode)

	return promptSubmissionContext{
		Language:         strings.TrimSpace(input.Language),
		Mode:             strings.TrimSpace(input.Mode),
		Code:             cleanCode,
		NumberedCode:     trimText(numberCode(numberCodeInput{Code: cleanCode, MaxLines: 260}), maxPromptNumberedRunes),
		CodeSignals:      signals,
		ApparentApproach: inferApproachFromSignals(signals),
	}
}

type numberCodeInput struct {
	Code     string
	MaxLines int
}

func numberCode(input numberCodeInput) string {
	cleanCode := strings.TrimSpace(input.Code)
	if cleanCode == "" {
		return ""
	}

	codeLines := strings.Split(cleanCode, "\n")
	maxLines := input.MaxLines
	if maxLines <= 0 {
		maxLines = len(codeLines)
	}

	var builder strings.Builder
	for lineIndex, codeLine := range codeLines {
		if lineIndex >= maxLines {
			builder.WriteString("\n... truncated ...")
			break
		}
		builder.WriteString(fmt.Sprintf("%d | %s", lineIndex+1, codeLine))
		if lineIndex < len(codeLines)-1 && lineIndex < maxLines-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func detectCodeSignals(code string) []string {
	if strings.TrimSpace(code) == "" {
		return []string{}
	}

	lowerCode := strings.ToLower(code)
	signals := []string{}
	signals = appendIfDetected(appendSignalInput{Signals: signals, Condition: strings.Contains(lowerCode, "queue<"), Label: "queue-based bfs pattern"})
	signals = appendIfDetected(appendSignalInput{Signals: signals, Condition: strings.Contains(lowerCode, "priority_queue<"), Label: "priority-queue shortest path / greedy pattern"})
	signals = appendIfDetected(appendSignalInput{Signals: signals, Condition: strings.Contains(lowerCode, "vector<vector"), Label: "2d grid/dp state representation"})
	signals = appendIfDetected(appendSignalInput{Signals: signals, Condition: strings.Contains(lowerCode, "unordered_map") || strings.Contains(lowerCode, "map<"), Label: "hash/map indexing strategy"})
	signals = appendIfDetected(appendSignalInput{Signals: signals, Condition: strings.Contains(lowerCode, "sort(") || strings.Contains(lowerCode, "std::sort("), Label: "sorting-based ordering"})
	signals = appendIfDetected(appendSignalInput{Signals: signals, Condition: strings.Contains(lowerCode, "while") || strings.Contains(lowerCode, "for ("), Label: "iterative traversal"})
	signals = appendIfDetected(appendSignalInput{Signals: signals, Condition: strings.Contains(lowerCode, "dfs(") || strings.Contains(lowerCode, "recurs"), Label: "recursive/dfs style"})

	return signals
}

type appendSignalInput struct {
	Signals   []string
	Condition bool
	Label     string
}

func appendIfDetected(input appendSignalInput) []string {
	if !input.Condition {
		return input.Signals
	}

	for _, signal := range input.Signals {
		if signal == input.Label {
			return input.Signals
		}
	}

	return append(input.Signals, input.Label)
}

func inferApproachFromSignals(signals []string) string {
	if len(signals) == 0 {
		return "not enough signal to infer approach"
	}

	if containsString(signals, "queue-based bfs pattern") {
		return "breadth-first search / state traversal"
	}
	if containsString(signals, "priority-queue shortest path / greedy pattern") {
		return "best-first / shortest-path style"
	}
	if containsString(signals, "recursive/dfs style") {
		return "depth-first recursive search"
	}

	return signals[0]
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

type promptResultContext struct {
	Overall          map[string]interface{}   `json:"overall"`
	Compile          map[string]interface{}   `json:"compile"`
	Timing           map[string]interface{}   `json:"timing"`
	Artifacts        map[string]interface{}   `json:"artifacts"`
	TestsSample      []map[string]interface{} `json:"tests_sample"`
	FirstFailingTest *promptFailingTestcase   `json:"first_failing_test"`
}

type promptFailingTestcase struct {
	TestcaseID int                  `json:"testcase_id"`
	Verdict    string               `json:"verdict"`
	RuntimeMs  int                  `json:"runtime_ms"`
	MemoryKb   int                  `json:"memory_kb"`
	Detail     promptTestcaseDetail `json:"detail"`
}

type promptTestcaseDetail struct {
	Input        string `json:"input,omitempty"`
	Expected     string `json:"expected,omitempty"`
	Actual       string `json:"actual,omitempty"`
	ExpectedHash string `json:"expected_hash,omitempty"`
	ActualHash   string `json:"actual_hash,omitempty"`
}

func buildPromptResultContext(resultPayload map[string]interface{}) promptResultContext {
	tests := extractTestsFromPayload(resultPayload)
	return promptResultContext{
		Overall:          mapFromInterface(resultPayload["overall"]),
		Compile:          mapFromInterface(resultPayload["compile"]),
		Timing:           mapFromInterface(resultPayload["timing"]),
		Artifacts:        mapFromInterface(resultPayload["artifacts"]),
		TestsSample:      summarizeTests(tests),
		FirstFailingTest: extractFirstFailingTestcase(tests),
	}
}

func extractTestsFromPayload(resultPayload map[string]interface{}) []interface{} {
	tests := arrayFromInterface(resultPayload["tests"])
	if len(tests) == 0 {
		return []interface{}{}
	}
	return tests
}

func summarizeTests(testRows []interface{}) []map[string]interface{} {
	if len(testRows) == 0 {
		return []map[string]interface{}{}
	}

	sample := make([]map[string]interface{}, 0, len(testRows))
	for _, testRow := range testRows {
		rowValues := mapFromInterface(testRow)
		if len(rowValues) == 0 {
			continue
		}

		summary := map[string]interface{}{
			"testcase_id": intFromInterface(rowValues["testcase_id"]),
			"verdict":     trimText(stringFromInterface(rowValues["verdict"]), 32),
			"runtime_ms":  intFromInterface(rowValues["runtime_ms"]),
			"memory_kb":   intFromInterface(rowValues["memory_kb"]),
		}
		detailValues := mapFromInterface(rowValues["detail"])
		if len(detailValues) != 0 {
			summary["detail"] = map[string]interface{}{
				"input":         trimText(stringFromInterface(detailValues["input"]), 800),
				"expected":      trimText(stringFromInterface(detailValues["expected"]), 200),
				"actual":        trimText(stringFromInterface(detailValues["actual"]), 200),
				"expected_hash": trimText(stringFromInterface(detailValues["expected_hash"]), 80),
				"actual_hash":   trimText(stringFromInterface(detailValues["actual_hash"]), 80),
			}
		}

		sample = append(sample, summary)
		if len(sample) >= maxPromptTestsSample {
			break
		}
	}

	return sample
}

func extractFirstFailingTestcase(testRows []interface{}) *promptFailingTestcase {
	for _, testRow := range testRows {
		rowValues := mapFromInterface(testRow)
		if len(rowValues) == 0 {
			continue
		}

		verdict := strings.TrimSpace(stringFromInterface(rowValues["verdict"]))
		if verdict == "" || verdict == stores.VerdictAccepted || verdict == stores.VerdictSkipped {
			continue
		}

		detailValues := mapFromInterface(rowValues["detail"])
		return &promptFailingTestcase{
			TestcaseID: intFromInterface(rowValues["testcase_id"]),
			Verdict:    trimText(verdict, 32),
			RuntimeMs:  intFromInterface(rowValues["runtime_ms"]),
			MemoryKb:   intFromInterface(rowValues["memory_kb"]),
			Detail: promptTestcaseDetail{
				Input:        trimText(stringFromInterface(detailValues["input"]), 1400),
				Expected:     trimText(stringFromInterface(detailValues["expected"]), 280),
				Actual:       trimText(stringFromInterface(detailValues["actual"]), 280),
				ExpectedHash: trimText(stringFromInterface(detailValues["expected_hash"]), 80),
				ActualHash:   trimText(stringFromInterface(detailValues["actual_hash"]), 80),
			},
		}
	}

	return nil
}

func mapFromInterface(value interface{}) map[string]interface{} {
	mappedValue, ok := value.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return mappedValue
}

func arrayFromInterface(value interface{}) []interface{} {
	arrayValue, ok := value.([]interface{})
	if !ok {
		return []interface{}{}
	}
	return arrayValue
}

func stringFromInterface(value interface{}) string {
	if value == nil {
		return ""
	}

	switch casted := value.(type) {
	case string:
		return casted
	case json.Number:
		return casted.String()
	default:
		return fmt.Sprintf("%v", casted)
	}
}

func intFromInterface(value interface{}) int {
	switch casted := value.(type) {
	case float64:
		return int(casted)
	case float32:
		return int(casted)
	case int:
		return casted
	case int32:
		return int(casted)
	case int64:
		return int(casted)
	case uint:
		return int(casted)
	case uint32:
		return int(casted)
	case uint64:
		return int(casted)
	case json.Number:
		numberValue, _ := casted.Int64()
		return int(numberValue)
	default:
		return 0
	}
}

func extractVerdict(result map[string]interface{}) string {
	overall := mapFromInterface(result["overall"])
	verdict := strings.TrimSpace(stringFromInterface(overall["verdict"]))
	return verdict
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

const practicePromptPrefix = `You are an expert competitive-programming coach.
Respond ONLY with JSON matching this schema:
{
  "summary": "string",
  "root_causes": [{"type":"LOGIC_BUG","confidence":0.5,"code_refs":[{"start_line":1,"end_line":1}]}],
  "hints": [{"level":1,"text":"string"}],
  "complexity_feedback": {"detected":{"time":"O(n)","space":"O(n)"},"target":{"time":"O(n)","space":"O(n)"}},
  "next_actions": ["string"]
}

Mandatory reasoning order:
1) Read problem statement + constraints + examples.
2) Read submission code and infer intended approach from code signals.
3) Compare against editorial + official/reference solutions to spot gaps.
4) Use compile/tests evidence and first failing testcase to identify root causes.
5) Produce actionable hints that nudge the user to fix the current approach.

Rules:
- Do not give full code unless policy.allow_full_solution is true.
- Do not hallucinate hidden testcase contents.
- Keep hints specific to this submission, not generic.
Payload:`
