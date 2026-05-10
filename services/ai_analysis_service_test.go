package services

import (
	"strings"
	"testing"

	"github.com/mcpx/boilerplate/stores"
)

func TestBuildPromptPayloadIncludesEditorialAndSignals(t *testing.T) {
	problemModel := &stores.ProblemModel{
		Title:         "Grid Shortest Path",
		Difficulty:    stores.ProblemDifficultyMedium,
		StatementJSON: stores.SerializeProblemStatement(stores.ProblemStatement{Markdown: "Find shortest path in a grid."}),
		IOSpecJSON:    `{"mode":"STDIN"}`,
		ConstraintsJSON: mustMarshalJSON(stores.ProblemConstraints{
			TimeLimitMs:   2000,
			MemoryLimitKb: 262144,
		}),
		TagsJSON:      stores.SerializeProblemTags([]string{"graph", "bfs"}),
		EditorialJSON: stores.SerializeProblemEditorial(stores.ProblemEditorial{Markdown: "Use BFS with state pruning."}),
		SolutionsJSON: `[{"language":"cpp","approach_summary":"BFS level traversal","code":"int main(){return 0;}"}]`,
	}

	referenceSolutions := []stores.SolutionModel{{
		Language:        "cpp",
		ApproachSummary: "Track remaining eliminations in visited state.",
		ComplexityJSON:  `{"time":"O(m*n*k)","space":"O(m*n*k)"}`,
		CodeText:        "int solve(){return 0;}",
		IsReference:     true,
	}}

	payload := buildPromptPayload(practicePromptInput{
		Policy:             AnalysisPolicy{Mode: "COACH", HintLevel: 2, AllowFullSolution: false},
		Problem:            problemModel,
		ReferenceSolutions: referenceSolutions,
		Submission: promptSubmissionInput{
			Language: "cpp",
			Mode:     stores.SubmissionModeRun,
			Code:     "queue<int> q;\nvector<vector<int>> seen;\nwhile(!q.empty()){}",
		},
		ResultPayload: map[string]interface{}{
			"overall": map[string]interface{}{"verdict": stores.VerdictWrongAnswer},
			"tests": []interface{}{
				map[string]interface{}{
					"testcase_id": 7,
					"verdict":     stores.VerdictWrongAnswer,
					"runtime_ms":  5,
					"detail": map[string]interface{}{
						"input":    "4 4 0",
						"expected": "6",
						"actual":   "-1",
					},
				},
			},
		},
	})

	if payload.Problem.EditorialMarkdown == "" {
		t.Fatalf("expected editorial markdown in prompt payload")
	}
	if len(payload.Problem.OfficialSolutions) == 0 {
		t.Fatalf("expected official solutions in prompt payload")
	}
	if len(payload.Problem.ReferenceSolutions) == 0 {
		t.Fatalf("expected reference solutions in prompt payload")
	}
	if len(payload.Submission.CodeSignals) == 0 {
		t.Fatalf("expected code signals in prompt payload")
	}
	if !containsString(payload.Submission.CodeSignals, "queue-based bfs pattern") {
		t.Fatalf("expected bfs signal, got %v", payload.Submission.CodeSignals)
	}
	if !strings.Contains(payload.Submission.NumberedCode, "1 |") {
		t.Fatalf("expected numbered code to include line numbers")
	}
	if payload.Result.FirstFailingTest == nil {
		t.Fatalf("expected first failing testcase snapshot")
	}
}

func TestExtractFirstFailingTestcase(t *testing.T) {
	testRows := []interface{}{
		map[string]interface{}{"testcase_id": 1, "verdict": stores.VerdictAccepted},
		map[string]interface{}{
			"testcase_id": 2,
			"verdict":     stores.VerdictWrongAnswer,
			"runtime_ms":  10,
			"memory_kb":   0,
			"detail": map[string]interface{}{
				"input":    "2 2",
				"expected": "4",
				"actual":   "3",
			},
		},
		map[string]interface{}{"testcase_id": 3, "verdict": stores.VerdictRuntimeError},
	}

	firstFailure := extractFirstFailingTestcase(testRows)
	if firstFailure == nil {
		t.Fatalf("expected failing testcase")
	}
	if firstFailure.TestcaseID != 2 {
		t.Fatalf("expected testcase id 2, got %d", firstFailure.TestcaseID)
	}
	if firstFailure.Verdict != stores.VerdictWrongAnswer {
		t.Fatalf("expected wrong answer verdict, got %s", firstFailure.Verdict)
	}
}

func TestBuildAnalysisFingerprintIncludesProblemContext(t *testing.T) {
	submissionModel := &stores.SubmissionModel{ID: 11, CodeText: "int main(){return 0;}"}
	resultModel := &stores.SubmissionResultModel{OverallJSON: `{"verdict":"AC"}`, CompileJSON: `{"verdict":"AC"}`}

	firstContext := analysisContext{
		Submission: submissionModel,
		Result:     resultModel,
		Problem: &stores.ProblemModel{
			Title:         "Sample",
			StatementJSON: stores.SerializeProblemStatement(stores.ProblemStatement{Markdown: "A"}),
			EditorialJSON: stores.SerializeProblemEditorial(stores.ProblemEditorial{Markdown: "Editorial A"}),
		},
		Policy: AnalysisPolicy{Mode: "COACH", HintLevel: 2},
	}

	secondContext := analysisContext{
		Submission: submissionModel,
		Result:     resultModel,
		Problem: &stores.ProblemModel{
			Title:         "Sample",
			StatementJSON: stores.SerializeProblemStatement(stores.ProblemStatement{Markdown: "A"}),
			EditorialJSON: stores.SerializeProblemEditorial(stores.ProblemEditorial{Markdown: "Editorial B"}),
		},
		Policy: AnalysisPolicy{Mode: "COACH", HintLevel: 2},
	}

	firstFingerprint := buildAnalysisFingerprint(firstContext)
	secondFingerprint := buildAnalysisFingerprint(secondContext)

	if firstFingerprint.ResultHash == secondFingerprint.ResultHash {
		t.Fatalf("expected result hash to change when editorial context changes")
	}
}
