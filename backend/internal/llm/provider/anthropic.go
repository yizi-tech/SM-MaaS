package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mass-platform/backend/pkg/logging"
	"github.com/sony/gobreaker"
)

// AnthropicProvider implements the Provider interface for Anthropic's API.
type AnthropicProvider struct {
	baseURL    string
	apiKey     string
	apiVersion string
	httpClient *http.Client
	// streamClient has NO total Timeout (see openai.go for rationale): a total
	// deadline would hard-cut long SSE streams mid-response. Only the
	// time-to-response-header is bounded.
	streamClient *http.Client
	breaker      *gobreaker.CircuitBreaker
}

// ---------- Anthropic API request / response types ----------

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content,omitempty"`
}

type anthropicChatRequest struct {
	Model       string              `json:"model"`
	Messages    []anthropicMessage  `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
	System      string              `json:"system,omitempty"`
	Tools       []anthropicTool     `json:"tools,omitempty"`
	ToolChoice  json.RawMessage     `json:"tool_choice,omitempty"`
}

// anthropicTool mirrors the Anthropic tools[] entry format.
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicChatResponse struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Role         string                 `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	Model        string                 `json:"model"`
	Usage        anthropicUsage         `json:"usage"`
	StopReason   string                 `json:"stop_reason"`
	StopSequence *string                `json:"stop_sequence"`
}

type anthropicModelInfo struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Display   string `json:"display"`
	CreatedAt string `json:"created_at"`
}

type anthropicModelsResponse struct {
	Data []anthropicModelInfo `json:"data"`
}

type anthropicErrorResponse struct {
	Type  string             `json:"type"`
	Error anthropicErrorBody `json:"error"`
}

type anthropicErrorBody struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ---------- SSE stream event types ----------

type anthropicStreamContentBlockDelta struct {
	Type  string              `json:"type"`
	Index int                 `json:"index"`
	Delta *anthropicTextDelta `json:"delta"`
}

type anthropicTextDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json"`
}

type anthropicStreamMessageDelta struct {
	Type  string              `json:"type"`
	Delta *anthropicStopDelta `json:"delta"`
	Usage *anthropicUsage     `json:"usage"`
}

type anthropicStopDelta struct {
	StopReason   string  `json:"stop_reason"`
	StopSequence *string `json:"stop_sequence"`
}

type anthropicStreamMessageStart struct {
	Type    string                `json:"type"`
	Message *anthropicChatResponse `json:"message"`
}

// ---------- Constructor ----------

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(baseURL, apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		baseURL:    NormalizeBaseURL(baseURL),
		apiKey:     apiKey,
		apiVersion: "2023-06-01",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		streamClient: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				ResponseHeaderTimeout: 60 * time.Second,
			},
		},
		breaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "anthropic",
			Timeout: 60 * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return counts.Requests >= 3 && failureRatio >= 0.6
			},
		}),
	}
}

// ---------- Provider interface ----------

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// ChatCompletion sends a chat completion request to Anthropic.
func (p *AnthropicProvider) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	anthropicReq := p.toAnthropicChatRequest(req)

	body, err := p.doRequest(ctx, http.MethodPost, "/v1/messages", anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat completion: %w", err)
	}

	var anthropicResp anthropicChatResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		logging.Error("AnthropicProvider", "ChatCompletion", "failed to parse response", err, nil)
		return nil, fmt.Errorf("anthropic chat completion: parse response: %w", err)
	}

	return p.toChatCompletionResponse(&anthropicResp), nil
}

// ChatCompletionStream sends a streaming chat completion request to Anthropic.
func (p *AnthropicProvider) ChatCompletionStream(ctx context.Context, req *ChatCompletionRequest) (<-chan StreamEvent, error) {
	anthropicReq := p.toAnthropicChatRequest(req)
	anthropicReq.Stream = true

	stream, err := p.doStreamRequest(ctx, "/v1/messages", anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat completion stream: %w", err)
	}

	ch := make(chan StreamEvent)
	go p.readSSEStream(ctx, stream, ch)
	return ch, nil
}

// Completion sends a text completion request via the messages endpoint.
func (p *AnthropicProvider) Completion(ctx context.Context, req *CompletionRequest) (*ChatCompletionResponse, error) {
	chatReq := &ChatCompletionRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
		Messages: []Message{
			{Role: "user", Content: ContentString(req.Prompt)},
		},
	}
	return p.ChatCompletion(ctx, chatReq)
}

// CompletionStream sends a streaming text completion via the messages endpoint.
func (p *AnthropicProvider) CompletionStream(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error) {
	chatReq := &ChatCompletionRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
		Messages: []Message{
			{Role: "user", Content: ContentString(req.Prompt)},
		},
	}
	return p.ChatCompletionStream(ctx, chatReq)
}

// ListModels retrieves available models from Anthropic.
func (p *AnthropicProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	body, err := p.doRequest(ctx, http.MethodGet, "/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("anthropic list models: %w", err)
	}

	var modelsResp anthropicModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		logging.Error("AnthropicProvider", "ListModels", "failed to parse response", err, nil)
		return nil, fmt.Errorf("anthropic list models: parse response: %w", err)
	}

	result := make([]ModelInfo, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		result = append(result, ModelInfo{
			ID:       m.ID,
			Object:   m.Type,
			OwnedBy:  "anthropic",
			Provider: "anthropic",
		})
	}
	return result, nil
}

// ---------- HTTP helpers ----------

func (p *AnthropicProvider) doRequest(ctx context.Context, method, path string, reqBody interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	p.setHeaders(httpReq)

	var respBody []byte
	_, err = p.breaker.Execute(func() (interface{}, error) {
		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			logging.Error("AnthropicProvider", "doRequest", "http request failed", err, map[string]interface{}{
				"method": method,
				"path":   path,
			})
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response body: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := parseAnthropicError(body, resp.StatusCode)
			logging.Error("AnthropicProvider", "doRequest", "API error", apiErr, map[string]interface{}{
				"status": resp.StatusCode,
				"method": method,
				"path":   path,
			})
			return nil, apiErr
		}

		respBody = body
		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

func (p *AnthropicProvider) doStreamRequest(ctx context.Context, path string, reqBody interface{}) (io.ReadCloser, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}

	p.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.streamClient.Do(httpReq)
	if err != nil {
		logging.Error("AnthropicProvider", "doStreamRequest", "http request failed", err, map[string]interface{}{
			"path": path,
		})
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		apiErr := parseAnthropicError(body, resp.StatusCode)
		return nil, apiErr
	}

	return resp.Body, nil
}

func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", p.apiVersion)
	req.Header.Set("Content-Type", "application/json")
}

// ---------- SSE streaming ----------

func (p *AnthropicProvider) readSSEStream(ctx context.Context, stream io.ReadCloser, ch chan<- StreamEvent) {
	defer stream.Close()
	defer close(ch)

	// send delivers an event unless the caller gave up (context cancelled or
	// consumer went away). Prevents goroutine leaks on client disconnects.
	send := func(ev StreamEvent) bool {
		select {
		case ch <- ev:
			return true
		case <-ctx.Done():
			return false
		}
	}

	reader := bufio.NewReader(stream)
	var eventType string
	state := &anthropicStreamState{}

	for {
		select {
		case <-ctx.Done():
			send(StreamEvent{Error: ctx.Err()})
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				send(StreamEvent{Done: true})
				return
			}
			send(StreamEvent{Error: fmt.Errorf("read sse line: %w", err)})
			return
		}

		line = strings.TrimRight(line, "\r\n")

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if !p.handleSSEData(ctx, eventType, data, ch, send, state) {
				return
			}
			eventType = ""
			continue
		}

		// Ignore empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
	}
}

// anthropicStreamState tracks the content block index while streaming so
// tool_use blocks can be re-emitted as OpenAI-style tool_calls chunks.
type anthropicStreamState struct {
	blockIndex int
}

func (p *AnthropicProvider) handleSSEData(ctx context.Context, eventType, data string, ch chan<- StreamEvent, send func(StreamEvent) bool, state *anthropicStreamState) bool {
	switch eventType {
	case "message_start":
		// Optionally parse and forward the initial metadata; not required for content.
		var msgStart anthropicStreamMessageStart
		if err := json.Unmarshal([]byte(data), &msgStart); err != nil {
			logging.Error("AnthropicProvider", "handleSSEData", "failed to parse message_start", err, nil)
		}

	case "content_block_start":
		var start struct {
			Index int `json:"index"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(data), &start); err != nil {
			return true
		}
		if start.ContentBlock.Type == "tool_use" {
			state.blockIndex = start.Index
			chunk := fmt.Sprintf(`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":%s,"type":"function","function":{"name":%s,"arguments":""}}]}}]}`,
				strconv.Quote(start.ContentBlock.ID), strconv.Quote(start.ContentBlock.Name))
			return send(StreamEvent{Data: []byte(chunk)})
		}

	case "content_block_delta":
		var delta anthropicStreamContentBlockDelta
		if err := json.Unmarshal([]byte(data), &delta); err != nil {
			logging.Error("AnthropicProvider", "handleSSEData", "failed to parse content_block_delta", err, nil)
			return true
		}
		if delta.Delta == nil {
			return true
		}
		switch delta.Delta.Type {
		case "text_delta":
			if delta.Delta.Text != "" {
				return send(StreamEvent{Data: []byte(delta.Delta.Text)})
			}
		case "input_json_delta":
			if delta.Delta.PartialJSON != "" {
				chunk := fmt.Sprintf(`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":%s}}]}}]}`,
					strconv.Quote(delta.Delta.PartialJSON))
				return send(StreamEvent{Data: []byte(chunk)})
			}
		}

	case "message_delta":
		// Contains stop_reason and final usage — no text content to forward.

	case "message_stop":
		send(StreamEvent{Done: true})
		return false

	case "ping":
		// Anthropic sends periodic pings; ignore.

	default:
		// Unknown event type — ignore.
	}
	return true
}

// ---------- Conversion helpers ----------

func (p *AnthropicProvider) toAnthropicChatRequest(req *ChatCompletionRequest) *anthropicChatRequest {
	messages := make([]anthropicMessage, 0, len(req.Messages))
	systemPrompt := ""

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if systemPrompt != "" {
				systemPrompt += "\n" + msg.Content.Text()
			} else {
				systemPrompt = msg.Content.Text()
			}
			continue
		}
		// tool results become user messages with a tool_result block.
		if msg.Role == "tool" {
			blocks := []anthropicContentBlock{{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   json.RawMessage(strconv.Quote(msg.Content.Text())),
			}}
			messages = append(messages, anthropicMessage{
				Role:    "user",
				Content: mustMarshalJSON(blocks),
			})
			continue
		}
		// assistant tool calls become tool_use content blocks.
		if len(msg.ToolCalls) > 0 {
			var blocks []anthropicContentBlock
			if text := msg.Content.Text(); text != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: text})
			}
			for _, tc := range msg.ToolCalls {
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: json.RawMessage(tc.Function.Arguments),
				})
			}
			messages = append(messages, anthropicMessage{
				Role:    "assistant",
				Content: mustMarshalJSON(blocks),
			})
			continue
		}
		// Plain messages: keep the original representation (string or blocks).
		if raw, ok := msg.Content.value.(json.RawMessage); ok {
			messages = append(messages, anthropicMessage{Role: msg.Role, Content: raw})
			continue
		}
		messages = append(messages, anthropicMessage{
			Role:    msg.Role,
			Content: json.RawMessage(strconv.Quote(msg.Content.Text())),
		})
	}

	// If no messages remain after extracting system prompt, default to a user message.
	if len(messages) == 0 {
		messages = append(messages, anthropicMessage{Role: "user", Content: json.RawMessage(`"Hello"`)})
	}

	anthropicReq := &anthropicChatRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
		System:      systemPrompt,
		Tools:       toAnthropicTools(req.Tools),
		ToolChoice:  req.ToolChoice,
	}

	// Default max_tokens if not set
	if anthropicReq.MaxTokens <= 0 {
		anthropicReq.MaxTokens = 1024
	}

	return anthropicReq
}

// toAnthropicTools converts OpenAI-style tools[] into Anthropic tools[].
func toAnthropicTools(tools []Tool) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		if t.Type != "function" || t.Function.Name == "" {
			continue
		}
		schema := t.Function.Parameters
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		out = append(out, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: schema,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mustMarshalJSON(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func (p *AnthropicProvider) toChatCompletionResponse(anthropicResp *anthropicChatResponse) *ChatCompletionResponse {
	// Concatenate all text blocks and collect tool_use blocks.
	var contentBuilder strings.Builder
	var toolCalls []ToolCall
	for _, block := range anthropicResp.Content {
		switch block.Type {
		case "text":
			contentBuilder.WriteString(block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			})
		}
	}

	finishReason := anthropicResp.StopReason
	if finishReason == "" {
		finishReason = "stop"
	}

	message := Message{Role: "assistant", Content: ContentString(contentBuilder.String())}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	return &ChatCompletionResponse{
		ID:      anthropicResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   anthropicResp.Model,
		Choices: []Choice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: Usage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}
}

// ---------- Error handling ----------

func parseAnthropicError(body []byte, statusCode int) error {
	var errResp anthropicErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Errorf("anthropic API error (status %d): %s - %s", statusCode, errResp.Error.Type, errResp.Error.Message)
	}
	return fmt.Errorf("anthropic API error (status %d): %s", statusCode, string(body))
}