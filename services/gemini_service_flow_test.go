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

func TestGeminiGenerateMentorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}

		contents := payload["contents"].([]interface{})
		content := contents[0].(map[string]interface{})
		parts := content["parts"].([]interface{})
		text := parts[0].(map[string]interface{})["text"].(string)
		if !strings.Contains(text, "resume_text") {
			t.Fatalf("expected resume_text in prompt")
		}

		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": `{"summary":["One"],"strengths":["Two"],"gaps":["Three"],"plan_7d":["Day1"],"plan_30d":["Day30"],"resources":["Link"],"response_text":"Short response"}`},
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
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	output, err := client.GenerateMentorResponse(context.Background(), GeminiMentorInput{
		ResumeText: "Resume text",
		Analysis: GeminiResumeOutput{
			ATSScore:           80,
			ReadabilitySummary: "Clear",
			QuickWins:          []string{"Add metrics"},
		},
		Focus:    "Career Growth",
		Subfocus: "Promotion roadmap",
		Answers:  map[string]interface{}{"current_role": "Engineer"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.ResponseText == "" {
		t.Fatalf("expected response text")
	}
	if len(output.Plan7D) != 1 || output.Plan7D[0] != "Day1" {
		t.Fatalf("unexpected plan_7d")
	}
}

func TestGeminiGenerateChatResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}

		contents := payload["contents"].([]interface{})
		content := contents[0].(map[string]interface{})
		parts := content["parts"].([]interface{})
		text := parts[0].(map[string]interface{})["text"].(string)
		if !strings.Contains(text, "history") {
			t.Fatalf("expected history in prompt")
		}

		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": `{"reply_markdown":"Here is your answer."}`},
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
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	output, err := client.GenerateChatResponse(context.Background(), GeminiChatInput{
		ResumeText: "Resume text",
		Analysis: GeminiResumeOutput{
			ATSScore:           80,
			ReadabilitySummary: "Clear",
			QuickWins:          []string{"Add metrics"},
		},
		Focus:    "Career Growth",
		Subfocus: "Promotion roadmap",
		Answers:  map[string]interface{}{"current_role": "Engineer"},
		History:  []GeminiChatMessage{{Role: "user", Content: "Hello"}},
		Message:  "Next step?",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.ReplyMarkdown != "Here is your answer." {
		t.Fatalf("unexpected reply markdown")
	}
}

func TestGeminiTranscribeAudio(t *testing.T) {
	audio := []byte("audio-bytes")
	encoded := base64.StdEncoding.EncodeToString(audio)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}

		contents := payload["contents"].([]interface{})
		content := contents[0].(map[string]interface{})
		parts := content["parts"].([]interface{})
		inline := parts[1].(map[string]interface{})["inline_data"].(map[string]interface{})
		if inline["mime_type"].(string) != "audio/webm" {
			t.Fatalf("unexpected mime type")
		}
		if inline["data"].(string) != encoded {
			t.Fatalf("unexpected audio data")
		}

		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": `{"transcript":"Hello world"}`},
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
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	output, err := client.TranscribeAudio(context.Background(), GeminiTranscribeInput{
		Data:     audio,
		MimeType: "audio/webm",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.Transcript != "Hello world" {
		t.Fatalf("unexpected transcript")
	}
}
