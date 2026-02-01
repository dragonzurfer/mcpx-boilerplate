package services

import "testing"

func TestParseGeminiChatExtractsJSONFromPreamble(t *testing.T) {
	raw := []byte("{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Sure, here you go:\\n```json\\n{\\\"reply_markdown\\\":\\\"Hello there.\\\"}\\n```\\nLet me know if you need more.\"}]}}]}")

	output, err := parseGeminiChat(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.ReplyMarkdown != "Hello there." {
		t.Fatalf("unexpected reply_markdown: %q", output.ReplyMarkdown)
	}
	if output.Reply != "Hello there." {
		t.Fatalf("unexpected reply: %q", output.Reply)
	}
}

func TestParseGeminiChatExtractsJSONFromSecondPart(t *testing.T) {
	raw := []byte("{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Sure, here is the plan:\"},{\"text\":\"```json\\n{\\\"reply_markdown\\\":\\\"Second part.\\\"}\\n```\"}]}}]}")

	output, err := parseGeminiChat(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.ReplyMarkdown != "Second part." {
		t.Fatalf("unexpected reply_markdown: %q", output.ReplyMarkdown)
	}
}

func TestParseGeminiChatUsesNextCandidateWithJSON(t *testing.T) {
	raw := []byte("{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"\"}]}},{\"content\":{\"parts\":[{\"text\":\"```json\\n{\\\"reply_markdown\\\":\\\"From candidate 2\\\"}\\n```\"}]}}]}")

	output, err := parseGeminiChat(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.ReplyMarkdown != "From candidate 2" {
		t.Fatalf("unexpected reply_markdown: %q", output.ReplyMarkdown)
	}
}
