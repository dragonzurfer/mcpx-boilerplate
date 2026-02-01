package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	defaultGeminiModel   = "gemini-1.5-flash"
)

type ToolAIService interface {
	AnalyzeResume(ctx context.Context, input GeminiResumeInput) (GeminiResumeOutput, error)
	GenerateMentorResponse(ctx context.Context, input GeminiMentorInput) (GeminiMentorOutput, error)
	GenerateChatResponse(ctx context.Context, input GeminiChatInput) (GeminiChatOutput, error)
	TranscribeAudio(ctx context.Context, input GeminiTranscribeInput) (GeminiTranscribeOutput, error)
}

type GeminiClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

type GeminiClientInput struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

func NewGeminiClient(input GeminiClientInput) *GeminiClient {
	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = defaultGeminiModel
	}
	client := input.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &GeminiClient{
		apiKey:     strings.TrimSpace(input.APIKey),
		model:      model,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: client,
	}
}

type GeminiResumeInput struct {
	FileName string
	Data     []byte
	MimeType string
}

type GeminiResumeOutput struct {
	ATSScore           int      `json:"ats_score"`
	ReadabilitySummary string   `json:"readability_summary"`
	QuickWins          []string `json:"quick_wins"`
	ResumeText         string   `json:"resume_text"`
}

func (c *GeminiClient) AnalyzeResume(ctx context.Context, input GeminiResumeInput) (GeminiResumeOutput, error) {
	if err := c.ensureReady(); err != nil {
		return GeminiResumeOutput{}, err
	}
	if err := ensureBytes(input.Data, "resume file empty"); err != nil {
		return GeminiResumeOutput{}, err
	}
	mimeType := strings.TrimSpace(input.MimeType)
	if mimeType == "" {
		mimeType = "application/pdf"
	}

	payload := buildGeminiRequestPayload(geminiRequestInput{Prompt: resumePrompt, MimeType: mimeType, Data: input.Data})

	rawResponse, err := c.sendGenerateContent(ctx, payload)
	if err != nil {
		return GeminiResumeOutput{}, err
	}

	return parseGeminiResume(rawResponse)
}

const resumePrompt = `You are a resume reviewer. Respond ONLY with JSON:
{
  "ats_score": number (0-100),
  "readability_summary": string,
  "quick_wins": [string, string, string],
  "resume_text": string
}
Keep quick_wins short and actionable. resume_text should be the plain text extracted from the PDF.`

type GeminiMentorInput struct {
	ResumeText string
	Analysis   GeminiResumeOutput
	Focus      string
	Subfocus   string
	Answers    map[string]interface{}
}

type GeminiMentorOutput struct {
	Summary      []string `json:"summary"`
	Strengths    []string `json:"strengths"`
	Gaps         []string `json:"gaps"`
	Plan7D       []string `json:"plan_7d"`
	Plan30D      []string `json:"plan_30d"`
	Resources    []string `json:"resources"`
	ResponseText string   `json:"response_text"`
}

func (c *GeminiClient) GenerateMentorResponse(ctx context.Context, input GeminiMentorInput) (GeminiMentorOutput, error) {
	if err := c.ensureReady(); err != nil {
		return GeminiMentorOutput{}, err
	}
	prompt := buildMentorPrompt(input)
	payload := buildGeminiTextPayload(prompt)
	rawResponse, err := c.sendGenerateContent(ctx, payload)
	if err != nil {
		return GeminiMentorOutput{}, err
	}
	return parseGeminiMentor(rawResponse)
}

type GeminiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GeminiChatInput struct {
	ResumeText string
	Analysis   GeminiResumeOutput
	Focus      string
	Subfocus   string
	Answers    map[string]interface{}
	History    []GeminiChatMessage
	Message    string
}

type GeminiChatOutput struct {
	Reply        string `json:"reply"`
	ReplyMarkdown string `json:"reply_markdown"`
}

func (c *GeminiClient) GenerateChatResponse(ctx context.Context, input GeminiChatInput) (GeminiChatOutput, error) {
	if err := c.ensureReady(); err != nil {
		return GeminiChatOutput{}, err
	}
	prompt := buildChatPrompt(input)
	payload := buildGeminiTextPayload(prompt)
	rawResponse, err := c.sendGenerateContent(ctx, payload)
	if err != nil {
		return GeminiChatOutput{}, err
	}
	return parseGeminiChat(rawResponse)
}

type GeminiTranscribeInput struct {
	Data     []byte
	MimeType string
}

type GeminiTranscribeOutput struct {
	Transcript string `json:"transcript"`
}

func (c *GeminiClient) TranscribeAudio(ctx context.Context, input GeminiTranscribeInput) (GeminiTranscribeOutput, error) {
	if err := c.ensureReady(); err != nil {
		return GeminiTranscribeOutput{}, err
	}
	if err := ensureBytes(input.Data, "audio file empty"); err != nil {
		return GeminiTranscribeOutput{}, err
	}
	mimeType := strings.TrimSpace(input.MimeType)
	if mimeType == "" {
		mimeType = "audio/webm"
	}
	payload := buildGeminiRequestPayload(geminiRequestInput{
		Prompt:   transcribePrompt,
		MimeType: mimeType,
		Data:     input.Data,
	})
	rawResponse, err := c.sendGenerateContent(ctx, payload)
	if err != nil {
		return GeminiTranscribeOutput{}, err
	}
	return parseGeminiTranscribe(rawResponse)
}

type geminiRequestInput struct {
	Prompt   string
	MimeType string
	Data     []byte
}

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inline_data,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiGenerationConfig struct {
	ResponseMimeType string  `json:"responseMimeType"`
	Temperature      float32 `json:"temperature"`
}

func buildGeminiRequestPayload(input geminiRequestInput) geminiRequest {
	encoded := base64.StdEncoding.EncodeToString(input.Data)
	parts := []geminiPart{
		{Text: strings.TrimSpace(input.Prompt)},
		{InlineData: &geminiInlineData{MimeType: input.MimeType, Data: encoded}},
	}
	return buildGeminiRequest(parts)
}

func (c *GeminiClient) sendGenerateContent(ctx context.Context, payload geminiRequest) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := c.baseURL + "/models/" + c.model + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("gemini request failed")
	}

	return readResponseBody(resp.Body)
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func parseGeminiResume(raw []byte) (GeminiResumeOutput, error) {
	text, err := parseGeminiText(raw)
	if err != nil {
		return GeminiResumeOutput{}, err
	}
	output := GeminiResumeOutput{}
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return GeminiResumeOutput{}, err
	}
	if output.QuickWins == nil {
		output.QuickWins = []string{}
	}
	output.ResumeText = trimText(output.ResumeText, 100000)
	return output, nil
}

func parseGeminiMentor(raw []byte) (GeminiMentorOutput, error) {
	text, err := parseGeminiText(raw)
	if err != nil {
		return GeminiMentorOutput{}, err
	}
	output := GeminiMentorOutput{}
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return GeminiMentorOutput{}, err
	}
	output.Summary = ensureStringSlice(output.Summary)
	output.Strengths = ensureStringSlice(output.Strengths)
	output.Gaps = ensureStringSlice(output.Gaps)
	output.Plan7D = ensureStringSlice(output.Plan7D)
	output.Plan30D = ensureStringSlice(output.Plan30D)
	output.Resources = ensureStringSlice(output.Resources)
	output.ResponseText = strings.TrimSpace(output.ResponseText)
	return output, nil
}

func parseGeminiChat(raw []byte) (GeminiChatOutput, error) {
	text, err := parseGeminiText(raw)
	if err != nil {
		return GeminiChatOutput{}, err
	}
	output := GeminiChatOutput{}
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return GeminiChatOutput{}, err
	}
	output.ReplyMarkdown = strings.TrimSpace(output.ReplyMarkdown)
	output.Reply = strings.TrimSpace(output.Reply)
	if output.ReplyMarkdown == "" && output.Reply == "" {
		return GeminiChatOutput{}, errors.New("empty reply")
	}
	if output.ReplyMarkdown == "" {
		output.ReplyMarkdown = output.Reply
	}
	if output.Reply == "" {
		output.Reply = output.ReplyMarkdown
	}
	return output, nil
}

func parseGeminiTranscribe(raw []byte) (GeminiTranscribeOutput, error) {
	text, err := parseGeminiText(raw)
	if err != nil {
		return GeminiTranscribeOutput{}, err
	}
	output := GeminiTranscribeOutput{}
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return GeminiTranscribeOutput{}, err
	}
	output.Transcript = strings.TrimSpace(output.Transcript)
	if output.Transcript == "" {
		return GeminiTranscribeOutput{}, errors.New("empty transcript")
	}
	return output, nil
}

func buildGeminiTextPayload(prompt string) geminiRequest {
	return buildGeminiRequest([]geminiPart{{Text: strings.TrimSpace(prompt)}})
}

func buildGeminiRequest(parts []geminiPart) geminiRequest {
	return geminiRequest{
		Contents: []geminiContent{{Parts: parts}},
		GenerationConfig: geminiGenerationConfig{
			ResponseMimeType: "application/json",
			Temperature:      0.2,
		},
	}
}

func parseGeminiText(raw []byte) (string, error) {
	var response geminiResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", err
	}
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("gemini response empty")
	}
	text := strings.TrimSpace(response.Candidates[0].Content.Parts[0].Text)
	text = trimJSONFence(text)
	if text == "" {
		return "", errors.New("gemini text empty")
	}
	return text, nil
}

func trimJSONFence(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	return strings.TrimSpace(value)
}

func readResponseBody(reader io.Reader) ([]byte, error) {
	return io.ReadAll(reader)
}

func (c *GeminiClient) ensureReady() error {
	if c == nil || c.apiKey == "" {
		return errors.New("gemini api key missing")
	}
	return nil
}

func ensureBytes(value []byte, message string) error {
	if len(value) == 0 {
		return errors.New(message)
	}
	return nil
}

func ensureStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func trimText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

type mentorPromptContext struct {
	ResumeText         string                 `json:"resume_text"`
	ATSScore           int                    `json:"ats_score"`
	ReadabilitySummary string                 `json:"readability_summary"`
	QuickWins          []string               `json:"quick_wins"`
	Focus              string                 `json:"focus"`
	Subfocus           string                 `json:"subfocus"`
	Answers            map[string]interface{} `json:"answers"`
}

func buildMentorPrompt(input GeminiMentorInput) string {
	context := mentorPromptContext{
		ResumeText:         trimText(input.ResumeText, 8000),
		ATSScore:           input.Analysis.ATSScore,
		ReadabilitySummary: input.Analysis.ReadabilitySummary,
		QuickWins:          input.Analysis.QuickWins,
		Focus:              input.Focus,
		Subfocus:           input.Subfocus,
		Answers:            input.Answers,
	}
	encoded := marshalPromptJSON(context)
	return mentorPrompt + "\nContext:\n" + encoded
}

const mentorPrompt = `You are a career mentor. Use the context JSON to craft a response.
Respond ONLY with JSON:
{
  "summary": [string, string, string],
  "strengths": [string],
  "gaps": [string],
  "plan_7d": [string],
  "plan_30d": [string],
  "resources": [string],
  "response_text": string
}
Keep response_text under 200 words.`

type chatPromptContext struct {
	ResumeText         string                 `json:"resume_text"`
	ATSScore           int                    `json:"ats_score"`
	ReadabilitySummary string                 `json:"readability_summary"`
	QuickWins          []string               `json:"quick_wins"`
	Focus              string                 `json:"focus"`
	Subfocus           string                 `json:"subfocus"`
	Answers            map[string]interface{} `json:"answers"`
	History            []GeminiChatMessage    `json:"history"`
	Message            string                 `json:"message"`
}

func buildChatPrompt(input GeminiChatInput) string {
	context := chatPromptContext{
		ResumeText:         trimText(input.ResumeText, 6000),
		ATSScore:           input.Analysis.ATSScore,
		ReadabilitySummary: input.Analysis.ReadabilitySummary,
		QuickWins:          input.Analysis.QuickWins,
		Focus:              input.Focus,
		Subfocus:           input.Subfocus,
		Answers:            input.Answers,
		History:            trimChatHistory(input.History, 8),
		Message:            strings.TrimSpace(input.Message),
	}
	encoded := marshalPromptJSON(context)
	return chatPrompt + "\nContext:\n" + encoded
}

const chatPrompt = `You are a career copilot. Use the context JSON and reply to the latest user message.
Respond ONLY with JSON:
{ "reply_markdown": string }
Reply_markdown should be markdown (bullets, bold, links, short paragraphs). Keep it concise and action-oriented.`

const transcribePrompt = `Transcribe the audio to text. Respond ONLY with JSON:
{ "transcript": string }`

func marshalPromptJSON(value interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func trimChatHistory(history []GeminiChatMessage, limit int) []GeminiChatMessage {
	if limit <= 0 || len(history) == 0 {
		return []GeminiChatMessage{}
	}
	if len(history) <= limit {
		return history
	}
	return history[len(history)-limit:]
}
