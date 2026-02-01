package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiAnalyzeResumeParsesJSON(t *testing.T) {
	requestBody := []byte("%PDF-1.4 test")
	encoded := base64.StdEncoding.EncodeToString(requestBody)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "test-key" {
			t.Fatalf("expected api key header, got %q", got)
		}
		if !strings.Contains(r.URL.Path, "/models/gemini-1.5-flash:generateContent") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}

		contents := payload["contents"].([]interface{})
		content := contents[0].(map[string]interface{})
		parts := content["parts"].([]interface{})
		inline := parts[1].(map[string]interface{})["inline_data"].(map[string]interface{})
		if inline["mime_type"].(string) != "application/pdf" {
			t.Fatalf("expected pdf mime type")
		}
		if inline["data"].(string) != encoded {
			t.Fatalf("expected base64 data to match")
		}

		generation := payload["generationConfig"].(map[string]interface{})
		if generation["responseMimeType"].(string) != "application/json" {
			t.Fatalf("expected responseMimeType application/json")
		}

		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": `{"ats_score":82,"readability_summary":"Clear","quick_wins":["Add metrics"],"resume_text":"Plain resume text"}`},
						},
					},
				},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response failed: %v", err)
		}
	}))
	defer server.Close()

	client := NewGeminiClient(GeminiClientInput{
		APIKey:     "test-key",
		Model:      "gemini-1.5-flash",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	output, err := client.AnalyzeResume(context.Background(), GeminiResumeInput{
		FileName: "resume.pdf",
		Data:     requestBody,
		MimeType: "application/pdf",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.ATSScore != 82 {
		t.Fatalf("expected score 82, got %d", output.ATSScore)
	}
	if output.ReadabilitySummary != "Clear" {
		t.Fatalf("unexpected readability summary")
	}
	if len(output.QuickWins) != 1 || output.QuickWins[0] != "Add metrics" {
		t.Fatalf("unexpected quick wins")
	}
	if output.ResumeText != "Plain resume text" {
		t.Fatalf("unexpected resume text")
	}
}
