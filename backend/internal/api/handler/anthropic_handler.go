package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mass-platform/backend/internal/llm/provider"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/internal/monitor"
	"github.com/mass-platform/backend/pkg/logging"
	"github.com/mass-platform/backend/pkg/response"
)

// ---------- Anthropic Messages API types (downstream, client-facing) ----------

// anthropicMessagesRequest is the Anthropic Messages API request format.
type anthropicMessagesRequest struct {
	Model          string             `json:"model"`
	MaxTokens      int                `json:"max_tokens"`
	System         anthropicBlockText `json:"system,omitempty"`
	Messages       []anthropicMsg     `json:"messages"`
	Temperature    *float64           `json:"temperature,omitempty"`
	TopP           *float64           `json:"top_p,omitempty"`
	TopK           *int               `json:"top_k,omitempty"`
	StopSequences  []string           `json:"stop_sequences,omitempty"`
	Stream         bool               `json:"stream,omitempty"`
	MaxTokensInput json.RawMessage    `json:"-"`
	Tools          []anthropicReqTool `json:"tools,omitempty"`
	ToolChoice     json.RawMessage    `json:"tool_choice,omitempty"`
}

// anthropicReqTool is a tool definition in the Anthropic Messages format.
type anthropicReqTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// anthropicBlockText accepts either a plain string or an array of text blocks
// for the "system" field.
type anthropicBlockText struct {
	Set  bool
	Text string
}

// UnmarshalJSON accepts a string or [{type:"text",text:"..."}] blocks.
func (b *anthropicBlockText) UnmarshalJSON(data []byte) error {
	b.Set = true
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		b.Text = s
		return nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &blocks); err != nil {
		return fmt.Errorf("system must be a string or an array of text blocks")
	}
	var parts []string
	for _, blk := range blocks {
		if blk.Type == "text" {
			parts = append(parts, blk.Text)
		}
	}
	b.Text = strings.Join(parts, "\n")
	return nil
}

// anthropicMsg is a single message in the Anthropic format. Content may be a
// plain string or an array of content blocks (text / image / tool_result…).
type anthropicMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// contentToText extracts plain text from either a string or content blocks.
func contentToText(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, blk := range blocks {
		if blk.Type == "text" && blk.Text != "" {
			parts = append(parts, blk.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// anthropicMessageResponse is the non-streaming Anthropic Messages response.
type anthropicMessageResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Model        string                  `json:"model"`
	Content      []anthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        anthropicRespUsage      `json:"usage"`
}

type anthropicContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicRespUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// mapStopReason converts an OpenAI-style finish reason to an Anthropic
// stop_reason.
func mapStopReason(finish string) string {
	switch finish {
	case "", "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "stop_sequence":
		return "stop_sequence"
	case "tool_calls":
		return "tool_use"
	default:
		return finish
	}
}

func newAnthropicResponseID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// AnthropicMessages handles POST /v1/messages with the Anthropic Messages API
// format (Anthropic SDK compatible). The request is translated into the
// internal chat format, routed/billed like any other gateway call, and the
// response is translated back into Anthropic format (including SSE events for
// streaming).
func (h *LLMHandler) AnthropicMessages(c *gin.Context) {
	requestID := getRequestID(c)

	apiKey, keyExists := getAPIKeyFromContext(c)
	userID, userExists := getUserIDFromContext(c)
	if !userExists {
		response.AnthropicError(c, http.StatusUnauthorized, "authentication_error", "user not authenticated")
		return
	}

	var req anthropicMessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.Error("llm_handler", "anthropic_messages", "failed to parse request body", err,
			map[string]interface{}{"request_id": requestID})
		response.AnthropicError(c, http.StatusBadRequest, "invalid_request_error", "invalid request body")
		return
	}

	if req.Model == "" {
		response.AnthropicError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	if len(req.Model) > 100 {
		response.AnthropicError(c, http.StatusBadRequest, "invalid_request_error", "model name too long")
		return
	}
	if len(req.Messages) == 0 {
		response.AnthropicError(c, http.StatusBadRequest, "invalid_request_error", "messages is required")
		return
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 1024
	}

	if keyExists && !h.checkKeyModelAccess(apiKey, req.Model) {
		response.AnthropicError(c, http.StatusForbidden, "permission_error", "model not allowed for this api key")
		return
	}

	llmProvider, providerName := h.resolveProviderForModel(c.Request.Context(), req.Model)
	if llmProvider == nil {
		logging.Error("llm_handler", "anthropic_messages", "provider not found", nil,
			map[string]interface{}{"request_id": requestID, "model": req.Model})
		response.AnthropicError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("no provider available for model: %s", req.Model))
		return
	}

	if err := h.billingService.EnsureModelPriced(req.Model); err != nil {
		logging.Warn("llm_handler", "anthropic_messages", "unpriced model rejected",
			map[string]interface{}{"request_id": requestID, "model": req.Model})
		response.AnthropicError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("model pricing not configured: %s", req.Model))
		return
	}

	logging.Info("llm_handler", "anthropic_messages", "processing request",
		map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"model":      req.Model,
			"provider":   providerName,
			"stream":     req.Stream,
		})

	// Translate Anthropic request -> internal chat format (also needed below
	// for TPM estimation).
	chatReq := anthropicToChatRequest(&req)

	// Rate limiting (API key limits take precedence; otherwise the active
	// plan's limits apply; unsubscribed callers get platform defaults).
	release, ok := h.enforceRateLimits(c, apiKey, userID, req.Model, providerName,
		estimateChatPromptTokens(chatReq.Messages, chatReq.MaxTokens),
		func(msg string) {
			response.AnthropicError(c, http.StatusTooManyRequests, "rate_limit_error", msg)
		})
	if !ok {
		return
	}
	defer release()

	var apiKeyID *uint
	if keyExists {
		if id, ok := getAPIKeyIDFromContext(c); ok {
			apiKeyID = id
		}
	}
	billingType, subID := h.resolveBillingUnlimited(userID, req.Model)

	// Pay-per-use callers need funds on hand; block before touching upstream.
	if h.blockIfUnfunded(c, userID, req.Model, providerName, billingType, subID) {
		return
	}

	if req.Stream {
		h.handleAnthropicStream(c, llmProvider, chatReq, providerName, userID, requestID, apiKeyID, billingType, subID)
		return
	}

	callStarted := time.Now()
	resp, err := llmProvider.ChatCompletion(c.Request.Context(), chatReq)
	if err != nil {
		monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "error").Inc()
		logging.Error("llm_handler", "anthropic_messages", "provider request failed", err,
			map[string]interface{}{"request_id": requestID, "user_id": userID, "model": req.Model, "provider": providerName})
		response.AnthropicError(c, http.StatusBadGateway, "internal_error", fmt.Sprintf("upstream service error (request_id: %s), please try again later", requestID))
		return
	}

	// Billing + conversation retention
	if resp.Usage.TotalTokens > 0 {
		rec, err := h.billingService.RecordUsage(
			c.Request.Context(),
			userID, requestID, req.Model, providerName,
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens,
			resp.Usage.CachedTokenCount(),
			resp.Usage.CacheWriteTokenCount(),
			billingType, subID, apiKeyID,
			0,
			time.Since(callStarted).Milliseconds(),
		)
		if err != nil {
			logging.Error("llm_handler", "billing", "failed to record usage", err,
				map[string]interface{}{"request_id": requestID, "user_id": userID, "model": req.Model})
		} else {
			h.saveConversation(userID, apiKeyID, requestID, req.Model, chatReq.Messages,
				resp.Choices, rec.Cost, false,
				resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CachedTokenCount())
		}
	}

	monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "success").Inc()
	monitor.LLMTokenTotal.WithLabelValues(req.Model, "prompt").Add(float64(resp.Usage.PromptTokens))
	monitor.LLMTokenTotal.WithLabelValues(req.Model, "completion").Add(float64(resp.Usage.CompletionTokens))

	c.JSON(http.StatusOK, anthropicFromChatResponse(resp))
}

// anthropicToChatRequest translates an Anthropic Messages request into the
// internal OpenAI-style chat request.
func anthropicToChatRequest(req *anthropicMessagesRequest) *provider.ChatCompletionRequest {
	out := &provider.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    make([]provider.Message, 0, len(req.Messages)+1),
		Temperature: 1,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}
	if req.Temperature != nil {
		out.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		out.TopP = *req.TopP
	}
	for _, t := range req.Tools {
		if t.Name == "" {
			continue
		}
		out.Tools = append(out.Tools, provider.Tool{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	if req.ToolChoice != nil {
		out.ToolChoice = req.ToolChoice
	}
	if req.System.Set && req.System.Text != "" {
		out.Messages = append(out.Messages, provider.Message{Role: "system", Content: provider.ContentString(req.System.Text)})
	}
	for _, m := range req.Messages {
		role := m.Role
		if role == "" {
			role = "user"
		}
		out.Messages = append(out.Messages, anthropicMessageToChat(m, role))
	}
	if len(out.Messages) == 0 {
		out.Messages = append(out.Messages, provider.Message{Role: "user", Content: provider.ContentString("Hello")})
	}
	return out
}

// anthropicMessageToChat converts a single Anthropic message into the
// internal chat format, preserving tool_use / tool_result blocks.
func anthropicMessageToChat(m anthropicMsg, role string) provider.Message {
	var blocks []struct {
		Type      string          `json:"type"`
		Text      string          `json:"text"`
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Input     json.RawMessage `json:"input"`
		ToolUseID string          `json:"tool_use_id"`
		Content   json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		// Plain string content.
		return provider.Message{Role: role, Content: provider.ContentString(contentToText(m.Content))}
	}
	var textParts []string
	var toolCalls []provider.ToolCall
	toolResultID := ""
	toolResultContent := ""
	for _, b := range blocks {
		switch b.Type {
		case "text":
			textParts = append(textParts, b.Text)
		case "tool_use":
			toolCalls = append(toolCalls, provider.ToolCall{
				ID:   b.ID,
				Type: "function",
				Function: provider.ToolCallFunction{
					Name:      b.Name,
					Arguments: string(b.Input),
				},
			})
		case "tool_result":
			toolResultID = b.ToolUseID
			var s string
			if json.Unmarshal(b.Content, &s) == nil {
				toolResultContent = s
			} else {
				toolResultContent = contentToText(b.Content)
			}
		}
	}
	if len(toolCalls) > 0 {
		msg := provider.Message{Role: "assistant", Content: provider.ContentString(strings.Join(textParts, "\n"))}
		msg.ToolCalls = toolCalls
		return msg
	}
	if toolResultID != "" {
		return provider.Message{Role: "tool", Content: provider.ContentString(toolResultContent), ToolCallID: toolResultID}
	}
	// Other block types (image etc.): preserve the raw representation.
	return provider.Message{Role: role, Content: provider.ContentRaw(m.Content)}
}

// anthropicFromChatResponse converts the internal chat response into the
// Anthropic Messages response format.
func anthropicFromChatResponse(resp *provider.ChatCompletionResponse) *anthropicMessageResponse {
	var blocks []anthropicContentBlock
	stopReason := "end_turn"
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		text := choice.Message.Content.Text()
		if text != "" {
			blocks = append(blocks, anthropicContentBlock{Type: "text", Text: text})
		}
		for _, tc := range choice.Message.ToolCalls {
			blocks = append(blocks, anthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			})
		}
		if len(choice.Message.ToolCalls) > 0 {
			stopReason = "tool_use"
		} else {
			stopReason = mapStopReason(choice.FinishReason)
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, anthropicContentBlock{Type: "text", Text: ""})
	}
	cached := resp.Usage.CachedTokenCount()
	return &anthropicMessageResponse{
		ID:           newAnthropicResponseID(),
		Type:         "message",
		Role:         "assistant",
		Model:        resp.Model,
		Content:      blocks,
		StopReason:   stopReason,
		StopSequence: nil,
		Usage: anthropicRespUsage{
			InputTokens:              resp.Usage.PromptTokens,
			OutputTokens:             resp.Usage.CompletionTokens,
			CacheCreationInputTokens: 0,
			CacheReadInputTokens:     cached,
		},
	}
}

// ---------- Streaming (Anthropic SSE events) ----------

// handleAnthropicStream streams provider chunks as Anthropic SSE events
// (message_start / content_block_start / content_block_delta /
// content_block_stop / message_delta / message_stop).
func (h *LLMHandler) handleAnthropicStream(
	c *gin.Context,
	llmProvider provider.Provider,
	req *provider.ChatCompletionRequest,
	providerName string,
	userID uint,
	requestID string,
	apiKeyID *uint,
	billingType model.BillingType,
	subID *uint,
) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	writeAnthropicEvent := func(event string, data interface{}) {
		payload, err := json.Marshal(data)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, payload)
		c.Writer.Flush()
	}

	startedAt := time.Now()
	streamCh, err := llmProvider.ChatCompletionStream(c.Request.Context(), req)
	if err != nil {
		monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "error").Inc()
		logging.Error("llm_handler", "anthropic_stream", "failed to start stream", err,
			map[string]interface{}{"request_id": requestID, "user_id": userID, "model": req.Model, "provider": providerName})
		writeAnthropicEvent("error", map[string]interface{}{
			"type": "error",
			"error": map[string]string{
				"type":    "internal_error",
				"message": fmt.Sprintf("upstream service error (request_id: %s), please try again later", requestID),
			},
		})
		return
	}

	// message_start with initial metadata
	writeAnthropicEvent("message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            newAnthropicResponseID(),
			"type":          "message",
			"role":          "assistant",
			"model":         req.Model,
			"content":       []interface{}{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": anthropicRespUsage{
				InputTokens:              0,
				OutputTokens:             0,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     0,
			},
		},
	})

	var lastUsage *provider.Usage
	var streamContent strings.Builder
	var ttftMs int64
	// Reasoning deltas are forwarded as a native Anthropic thinking block so
	// the model's raw internal trace (which often contains tool-invocation
	// markup such as <invoke>...</invoke>) never surfaces as visible assistant
	// text. Content-block indices are allocated lazily in arrival order
	// (thinking, then text, then tool_use blocks), each with its own index.
	thinkingIndex := -1
	textIndex := -1
	nextBlockIndex := 0
	toolBlockIndices := make(map[int]int)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logging.Error("llm_handler", "anthropic_stream", "response writer does not support flushing", nil,
			map[string]interface{}{"request_id": requestID})
		return
	}

	// emitText lazily starts the text block and streams a text delta.
	emitText := func(s string) {
		if textIndex < 0 {
			textIndex = nextBlockIndex
			nextBlockIndex++
			writeAnthropicEvent("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": textIndex,
				"content_block": map[string]interface{}{
					"type": "text",
					"text": "",
				},
			})
		}
		writeAnthropicEvent("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": textIndex,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": s,
			},
		})
	}

	for event := range streamCh {
		// Time to first upstream chunk (TTFT): only count chunks carrying data
		// (skips empty/keepalive events emitted right after connect).
		if ttftMs == 0 && len(event.Data) > 0 {
			ttftMs = time.Since(startedAt).Milliseconds()
		}
		select {
		case <-c.Request.Context().Done():
			logging.Info("llm_handler", "anthropic_stream", "client disconnected",
				map[string]interface{}{"request_id": requestID, "user_id": userID})
			return
		default:
		}

		if event.Error != nil {
			monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "error").Inc()
			logging.Error("llm_handler", "anthropic_stream", "stream error", event.Error,
				map[string]interface{}{"request_id": requestID, "user_id": userID, "model": req.Model})
			writeAnthropicEvent("error", map[string]interface{}{
				"type": "error",
				"error": map[string]string{
					"type":    "stream_error",
					"message": fmt.Sprintf("upstream service error (request_id: %s), please try again later", requestID),
				},
			})
			return
		}

		if event.Done {
			break
		}

		if len(event.Data) == 0 {
			continue
		}

		// Parse the chunk: extract usage from the last data event, the
		// text delta and any tool_calls (OpenAI-style chunks from the
		// provider layer).
		var chunk struct {
			Usage   *provider.Usage `json:"usage,omitempty"`
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		parsed := json.Unmarshal(event.Data, &chunk) == nil
		if parsed {
			if chunk.Usage != nil {
				lastUsage = chunk.Usage
			}
		}
		text := ""
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta

			// Reasoning content is never rendered as visible text: it is
			// forwarded as an Anthropic thinking block instead.
			if delta.ReasoningContent != "" {
				if thinkingIndex < 0 {
					thinkingIndex = nextBlockIndex
					nextBlockIndex++
					writeAnthropicEvent("content_block_start", map[string]interface{}{
						"type":  "content_block_start",
						"index": thinkingIndex,
						"content_block": map[string]interface{}{
							"type":     "thinking",
							"thinking": "",
						},
					})
				}
				writeAnthropicEvent("content_block_delta", map[string]interface{}{
					"type":  "content_block_delta",
					"index": thinkingIndex,
					"delta": map[string]interface{}{
						"type":     "thinking_delta",
						"thinking": delta.ReasoningContent,
					},
				})
			}

			if delta.Content != "" {
				text = delta.Content
			}

			// Tool call deltas -> Anthropic tool_use blocks.
			for _, tc := range delta.ToolCalls {
				anIndex, exists := toolBlockIndices[tc.Index]
				if !exists {
					anIndex = nextBlockIndex
					nextBlockIndex++
					toolBlockIndices[tc.Index] = anIndex
					writeAnthropicEvent("content_block_start", map[string]interface{}{
						"type":  "content_block_start",
						"index": anIndex,
						"content_block": map[string]interface{}{
							"type":  "tool_use",
							"id":    tc.ID,
							"name":  tc.Function.Name,
							"input": map[string]interface{}{},
						},
					})
				}
				if tc.Function.Arguments != "" {
					writeAnthropicEvent("content_block_delta", map[string]interface{}{
						"type":  "content_block_delta",
						"index": anIndex,
						"delta": map[string]interface{}{
							"type":         "input_json_delta",
							"partial_json": tc.Function.Arguments,
						},
					})
				}
			}
		}
		if text == "" && !parsed {
			// Anthropic upstream provider already forwards plain text
			text = string(event.Data)
		}
		if text == "" {
			continue
		}

		streamContent.WriteString(text)
		emitText(text)
	}

	// Emit content_block_stop in index order (thinking, text, tools).
	if thinkingIndex >= 0 {
		writeAnthropicEvent("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": thinkingIndex,
		})
	}
	if textIndex >= 0 {
		writeAnthropicEvent("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": textIndex,
		})
	}
	toolIdx := make([]int, 0, len(toolBlockIndices))
	for _, idx := range toolBlockIndices {
		toolIdx = append(toolIdx, idx)
	}
	sort.Ints(toolIdx)
	for _, idx := range toolIdx {
		writeAnthropicEvent("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": idx,
		})
	}

	stopReason := "end_turn"
	if len(toolBlockIndices) > 0 {
		stopReason = "tool_use"
	}
	outputTokens := 0
	if lastUsage != nil {
		if lastUsage.CompletionTokens > 0 {
			outputTokens = lastUsage.CompletionTokens
		}
	}
	writeAnthropicEvent("message_delta", map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]interface{}{
			"output_tokens": outputTokens,
		},
	})
	writeAnthropicEvent("message_stop", map[string]interface{}{
		"type": "message_stop",
	})
	flusher.Flush()

	// Billing after the stream completes
	if lastUsage != nil && lastUsage.TotalTokens > 0 {
		rec, err := h.billingService.RecordUsage(
			context.Background(),
			userID, requestID, req.Model, providerName,
			lastUsage.PromptTokens, lastUsage.CompletionTokens,
			lastUsage.CachedTokenCount(),
			lastUsage.CacheWriteTokenCount(),
			billingType, subID, apiKeyID,
			ttftMs,
			time.Since(startedAt).Milliseconds(),
		)
		if err != nil {
			logging.Error("llm_handler", "billing", "failed to record stream usage", err,
				map[string]interface{}{"request_id": requestID, "user_id": userID, "model": req.Model})
		} else {
			content := streamContent.String()
			h.saveConversation(userID, apiKeyID, requestID, req.Model, req.Messages,
				[]provider.Choice{{Message: provider.Message{Role: "assistant", Content: provider.ContentString(content)}}},
				rec.Cost, true, lastUsage.PromptTokens, lastUsage.CompletionTokens, lastUsage.CachedTokenCount())
		}
		monitor.LLMTokenTotal.WithLabelValues(req.Model, "prompt").Add(float64(lastUsage.PromptTokens))
		monitor.LLMTokenTotal.WithLabelValues(req.Model, "completion").Add(float64(lastUsage.CompletionTokens))
	}

	monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "success").Inc()
}
