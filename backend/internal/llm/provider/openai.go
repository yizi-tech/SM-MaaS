package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mass-platform/backend/pkg/logging"
	"github.com/sony/gobreaker"
)

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs.
type OpenAIProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	// streamClient has NO total Timeout: http.Client.Timeout is a deadline for
	// the whole request INCLUDING reading the streamed body, so a non-zero
	// value would hard-cut any SSE stream longer than it (long reasoning).
	// ResponseHeaderTimeout still protects against upstreams that accept the
	// request but never send headers back.
	streamClient *http.Client
	breaker      *gobreaker.CircuitBreaker
}

// NewOpenAIProvider creates a new OpenAIProvider with the given base URL and API key.
func NewOpenAIProvider(baseURL, apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		baseURL: NormalizeBaseURL(baseURL),
		apiKey:  apiKey,
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
			Name:        "openai-provider",
			MaxRequests: 3,
			Interval:    60 * time.Second,
			Timeout:     30 * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return counts.Requests >= 5 && failureRatio >= 0.6
			},
			OnStateChange: func(name string, from, to gobreaker.State) {
				logging.Info("openai_provider", "circuit_breaker",
					"state changed",
					map[string]interface{}{
						"name": name,
						"from": from.String(),
						"to":   to.String(),
					},
				)
			},
		}),
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// ChatCompletion sends a chat completion request and returns the response.
func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	result, err := p.breaker.Execute(func() (interface{}, error) {
		body, err := p.doRequest(ctx, http.MethodPost, "/v1/chat/completions", req)
		if err != nil {
			return nil, err
		}

		var resp ChatCompletionResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			logging.Error("openai_provider", "chat_completion",
				"failed to parse response", err,
				map[string]interface{}{"model": req.Model},
			)
			return nil, fmt.Errorf("openai: failed to parse chat completion response: %w", err)
		}
		// Reasoning models (e.g. DeepSeek-R1) return the visible text in
		// message.reasoning_content when message.content is empty. Merge it
		// into content so downstream clients always see the full reply — but
		// ONLY when the choice carries no tool_calls: a tool-call message
		// legitimately has empty content, and splicing the reasoning trace
		// into content would leak the model's raw internal text (which often
		// contains tool-invocation markup such as <invoke>...</invoke>) into
		// the visible reply.
		var raw struct {
			Choices []struct {
				Message struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(body, &raw) == nil {
			for i := range resp.Choices {
				if i < len(raw.Choices) && resp.Choices[i].Message.Content.Text() == "" &&
					raw.Choices[i].Message.ReasoningContent != "" && len(resp.Choices[i].Message.ToolCalls) == 0 {
					resp.Choices[i].Message.Content = ContentString(raw.Choices[i].Message.ReasoningContent)
				}
			}
		}
		return &resp, nil
	})

	if err != nil {
		return nil, err
	}
	return result.(*ChatCompletionResponse), nil
}

// ChatCompletionStream sends a streaming chat completion request and returns a channel of events.
func (p *OpenAIProvider) ChatCompletionStream(ctx context.Context, req *ChatCompletionRequest) (<-chan StreamEvent, error) {
	// Ensure stream is enabled
	reqCopy := *req
	reqCopy.Stream = true

	eventCh := make(chan StreamEvent, 100)

	result, err := p.breaker.Execute(func() (interface{}, error) {
		httpReq, err := p.buildRequest(ctx, http.MethodPost, "/v1/chat/completions", &reqCopy)
		if err != nil {
			return nil, err
		}

		httpResp, err := p.streamClient.Do(httpReq)
		if err != nil {
			logging.Error("openai_provider", "chat_completion_stream",
				"request failed", err,
				map[string]interface{}{"model": req.Model},
			)
			return nil, fmt.Errorf("openai: stream request failed: %w", err)
		}

		if httpResp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(httpResp.Body)
			httpResp.Body.Close()
			return nil, fmt.Errorf("openai: stream request returned status %d: %s", httpResp.StatusCode, string(bodyBytes))
		}

		return httpResp, nil
	})

	if err != nil {
		return nil, err
	}

	httpResp := result.(*http.Response)

	go p.readSSEStream(ctx, httpResp.Body, eventCh)

	return eventCh, nil
}

// Completion sends a text completion request and returns the response.
func (p *OpenAIProvider) Completion(ctx context.Context, req *CompletionRequest) (*ChatCompletionResponse, error) {
	result, err := p.breaker.Execute(func() (interface{}, error) {
		body, err := p.doRequest(ctx, http.MethodPost, "/v1/completions", req)
		if err != nil {
			return nil, err
		}

		var rawResp struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			Model   string `json:"model"`
			Choices []struct {
				Text         string `json:"text"`
				Index        int    `json:"index"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal(body, &rawResp); err != nil {
			logging.Error("openai_provider", "completion",
				"failed to parse response", err,
				map[string]interface{}{"model": req.Model},
			)
			return nil, fmt.Errorf("openai: failed to parse completion response: %w", err)
		}

		resp := &ChatCompletionResponse{
			ID:      rawResp.ID,
			Object:  rawResp.Object,
			Created: rawResp.Created,
			Model:   rawResp.Model,
			Usage: Usage{
				PromptTokens:     rawResp.Usage.PromptTokens,
				CompletionTokens: rawResp.Usage.CompletionTokens,
				TotalTokens:      rawResp.Usage.TotalTokens,
			},
		}
		for _, c := range rawResp.Choices {
			resp.Choices = append(resp.Choices, Choice{
				Index:        c.Index,
				Message:      Message{Role: "assistant", Content: ContentString(c.Text)},
				FinishReason: c.FinishReason,
			})
		}
		return resp, nil
	})

	if err != nil {
		return nil, err
	}
	return result.(*ChatCompletionResponse), nil
}

// CompletionStream sends a streaming text completion request and returns a channel of events.
func (p *OpenAIProvider) CompletionStream(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error) {
	reqCopy := *req
	reqCopy.Stream = true

	eventCh := make(chan StreamEvent, 100)

	result, err := p.breaker.Execute(func() (interface{}, error) {
		httpReq, err := p.buildRequest(ctx, http.MethodPost, "/v1/completions", &reqCopy)
		if err != nil {
			return nil, err
		}

		httpResp, err := p.streamClient.Do(httpReq)
		if err != nil {
			logging.Error("openai_provider", "completion_stream",
				"request failed", err,
				map[string]interface{}{"model": req.Model},
			)
			return nil, fmt.Errorf("openai: stream request failed: %w", err)
		}

		if httpResp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(httpResp.Body)
			httpResp.Body.Close()
			return nil, fmt.Errorf("openai: stream request returned status %d: %s", httpResp.StatusCode, string(bodyBytes))
		}

		return httpResp, nil
	})

	if err != nil {
		return nil, err
	}

	httpResp := result.(*http.Response)

	go p.readSSEStream(ctx, httpResp.Body, eventCh)

	return eventCh, nil
}

// ListModels returns the list of available models from the provider.
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	result, err := p.breaker.Execute(func() (interface{}, error) {
		body, err := p.doRequest(ctx, http.MethodGet, "/v1/models", nil)
		if err != nil {
			return nil, err
		}

		var rawResp struct {
			Data []struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				OwnedBy string `json:"owned_by"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &rawResp); err != nil {
			logging.Error("openai_provider", "list_models",
				"failed to parse response", err, nil,
			)
			return nil, fmt.Errorf("openai: failed to parse models list: %w", err)
		}

		models := make([]ModelInfo, 0, len(rawResp.Data))
		for _, m := range rawResp.Data {
			models = append(models, ModelInfo{
				ID:       m.ID,
				Object:   m.Object,
				Created:  m.Created,
				OwnedBy:  m.OwnedBy,
				Provider: "openai",
			})
		}
		return models, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]ModelInfo), nil
}

// doRequest is a helper that builds and executes an HTTP request, returning the response body.
func (p *OpenAIProvider) doRequest(ctx context.Context, method, path string, reqBody interface{}) ([]byte, error) {
	httpReq, err := p.buildRequest(ctx, method, path, reqBody)
	if err != nil {
		return nil, err
	}

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		logging.Error("openai_provider", "http_request",
			"request failed", err,
			map[string]interface{}{"method": method, "path": path},
		)
		return nil, fmt.Errorf("openai: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		logging.Error("openai_provider", "read_body",
			"failed to read response body", err,
			map[string]interface{}{"status": httpResp.StatusCode},
		)
		return nil, fmt.Errorf("openai: failed to read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: request returned status %d: %s", httpResp.StatusCode, string(body))
	}

	return body, nil
}

// buildRequest creates an HTTP request with the appropriate headers and body.
func (p *OpenAIProvider) buildRequest(ctx context.Context, method, path string, reqBody interface{}) (*http.Request, error) {
	url := p.baseURL + path

	var bodyReader io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			logging.Error("openai_provider", "marshal_request",
				"failed to marshal request body", err, nil,
			)
			return nil, fmt.Errorf("openai: failed to marshal request: %w", err)
		}
		bodyReader = strings.NewReader(string(jsonBody))
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		logging.Error("openai_provider", "new_request",
			"failed to create request", err,
			map[string]interface{}{"method": method, "url": url},
		)
		return nil, fmt.Errorf("openai: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	return httpReq, nil
}

// readSSEStream reads an SSE (Server-Sent Events) stream from the response body
// and sends parsed events to the provided channel. All sends are guarded by the
// caller's context: if the HTTP handler has already returned (e.g. client
// disconnected), the goroutine stops instead of blocking forever on a full
// channel, which would otherwise leak the goroutine and the TCP connection.
func (p *OpenAIProvider) readSSEStream(ctx context.Context, body io.ReadCloser, ch chan<- StreamEvent) {
	defer body.Close()
	defer close(ch)

	// send delivers an event unless the caller gave up (context cancelled or
	// channel closed by the consumer). Returns false to signal abort.
	send := func(ev StreamEvent) bool {
		select {
		case ch <- ev:
			return true
		case <-ctx.Done():
			return false
		}
	}

	scanner := bufio.NewScanner(body)
	// Increase buffer size for potentially large token payloads
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Check for context cancellation
		select {
		case <-ctx.Done():
			logging.Info("openai_provider", "sse_stream",
				"stream cancelled by context", map[string]interface{}{
					"error": ctx.Err().Error(),
				},
			)
			send(StreamEvent{Error: ctx.Err(), Done: true})
			return
		default:
		}

		// SSE format: "data: {...}" or "data: [DONE]"
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Check for the stream termination signal
			if data == "[DONE]" {
				send(StreamEvent{Done: true})
				return
			}

			if !send(StreamEvent{Data: []byte(data)}) {
				return
			}
		} else if line == "" {
			// Empty line indicates end of an SSE event
			continue
		}
		// Ignore other lines (e.g., "event:", "id:", "retry:")
	}

	if err := scanner.Err(); err != nil {
		// Check if the error is due to context cancellation
		if ctx.Err() != nil {
			send(StreamEvent{Error: ctx.Err(), Done: true})
			return
		}
		logging.Error("openai_provider", "sse_stream",
			"scanner error", err, nil,
		)
		send(StreamEvent{Error: fmt.Errorf("openai: stream read error: %w", err), Done: true})
	}
}
