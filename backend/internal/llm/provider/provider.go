package provider

import (
	"context"
	"encoding/json"
	"io"
	"strings"
)

// Message represents a chat message
type Message struct {
	Role       string       `json:"role"`
	Content    ContentValue `json:"content"`
	Name       string       `json:"name,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall   `json:"tool_calls,omitempty"`
}

// ContentValue holds a message's content in its original form. It accepts
// both the plain string form and the OpenAI multi-part array form
// ([{"type":"text","text":"..."}, ...]) and preserves whatever the client
// sent so the payload can be forwarded upstream unchanged.
type ContentValue struct {
	value any
}

// MarshalJSON re-emits the original content representation.
func (c ContentValue) MarshalJSON() ([]byte, error) {
	if c.value == nil {
		return []byte("null"), nil
	}
	if raw, ok := c.value.(json.RawMessage); ok {
		return raw, nil
	}
	return json.Marshal(c.value)
}

// UnmarshalJSON accepts a string, null, or any other JSON value (array form).
func (c *ContentValue) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		c.value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.value = s
		return nil
	}
	c.value = json.RawMessage(append([]byte(nil), data...))
	return nil
}

// Text renders the content as displayable text: the plain string itself, or
// the concatenated text parts of a multi-part array.
func (c ContentValue) Text() string {
	switch v := c.value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.RawMessage:
		var parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(v, &parts) == nil {
			var sb strings.Builder
			for _, p := range parts {
				if p.Text != "" {
					sb.WriteString(p.Text)
				}
			}
			return sb.String()
		}
		return ""
	default:
		return ""
	}
}

// ContentString builds a plain-string content value.
func ContentString(s string) ContentValue {
	return ContentValue{value: s}
}

// ContentRaw builds a content value from raw JSON (e.g. an array of
// multi-part blocks), preserving the exact representation for upstream
// passthrough.
func ContentRaw(raw json.RawMessage) ContentValue {
	return ContentValue{value: raw}
}

// NormalizeBaseURL strips a trailing "/v1" (plus surrounding slashes) from a
// provider base URL. All request paths already begin with "/v1", so accepting
// both "https://host" and "https://host/v1" avoids double "/v1" paths.
func NormalizeBaseURL(raw string) string {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if strings.HasSuffix(base, "/v1") {
		base = strings.TrimSuffix(base, "/v1")
	}
	return base
}

// ChatCompletionRequest represents a chat completion request
type ChatCompletionRequest struct {
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	User        string          `json:"user,omitempty"`
	// Tool-calling support (OpenAI compatible). Raw passthrough so the
	// gateway forwards schema details without interpreting them.
	Tools      []Tool          `json:"tools,omitempty"`
	ToolChoice json.RawMessage `json:"tool_choice,omitempty"`
}

// Tool describes a function the model may call (OpenAI tools[]).
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction holds the JSON schema of a callable function.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ToolCall is a function invocation emitted by the model.
type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction carries the invoked function name and JSON arguments.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens        int                 `json:"prompt_tokens"`
	CompletionTokens    int                 `json:"completion_tokens"`
	TotalTokens         int                 `json:"total_tokens"`
	CachedTokens        int                 `json:"cached_tokens,omitempty"`
	PromptTokensDetails *PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
	// Anthropic-style cache accounting.
	CacheCreationTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// PromptTokensDetails carries cache breakdowns returned by some gateways
// (OpenAI-style usage.prompt_tokens_details.cached_tokens).
type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// CachedTokenCount returns the number of cached (cache-hit) prompt tokens,
// honoring both the flat usage.cached_tokens field, the nested
// prompt_tokens_details, and the Anthropic cache_read_input_tokens field.
func (u Usage) CachedTokenCount() int {
	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
		return u.PromptTokensDetails.CachedTokens
	}
	if u.CacheReadTokens > 0 {
		return u.CacheReadTokens
	}
	return u.CachedTokens
}

// CacheWriteTokenCount returns the number of cache-creation (cache write)
// prompt tokens reported by the upstream (Anthropic-style).
func (u Usage) CacheWriteTokenCount() int {
	return u.CacheCreationTokens
}

// StreamEvent represents a streaming event
type StreamEvent struct {
	Data  []byte
	Error error
	Done  bool
}

// CompletionRequest represents a text completion request
type CompletionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
}

// ModelInfo represents information about a model
type ModelInfo struct {
	ID         string `json:"id"`
	Object     string `json:"object"`
	Created    int64  `json:"created"`
	OwnedBy    string `json:"owned_by"`
	Provider   string `json:"provider"`
}

// Provider defines the interface for LLM providers
type Provider interface {
	Name() string
	ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
	ChatCompletionStream(ctx context.Context, req *ChatCompletionRequest) (<-chan StreamEvent, error)
	Completion(ctx context.Context, req *CompletionRequest) (*ChatCompletionResponse, error)
	CompletionStream(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

// ProviderFactory creates provider instances based on configuration
type ProviderFactory struct {
	providers map[string]Provider
}

func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		providers: make(map[string]Provider),
	}
}

func (f *ProviderFactory) Register(name string, provider Provider) {
	f.providers[name] = provider
}

func (f *ProviderFactory) Get(name string) (Provider, bool) {
	p, ok := f.providers[name]
	return p, ok
}

func (f *ProviderFactory) GetAll() map[string]Provider {
	return f.providers
}

// ReadAll reads all content from a stream channel
func ReadAll(ch <-chan StreamEvent) ([]byte, error) {
	var result []byte
	for event := range ch {
		if event.Error != nil {
			return nil, event.Error
		}
		if event.Done {
			break
		}
		result = append(result, event.Data...)
	}
	return result, nil
}

// CopyStream copies from a stream channel to a writer
func CopyStream(w io.Writer, ch <-chan StreamEvent) error {
	for event := range ch {
		if event.Error != nil {
			return event.Error
		}
		if event.Done {
			break
		}
		if _, err := w.Write(event.Data); err != nil {
			return err
		}
	}
	return nil
}