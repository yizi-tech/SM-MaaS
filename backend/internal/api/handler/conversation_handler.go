package handler

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/internal/repository"
	"github.com/mass-platform/backend/pkg/response"
)

// ConversationHandler serves user-facing conversation retention (JSONL
// export) and program-issue feedback endpoints.
type ConversationHandler struct {
	convoRepo *repository.ConversationLogRepository
	fbRepo    *repository.FeedbackRepository
}

func NewConversationHandler(
	convoRepo *repository.ConversationLogRepository,
	fbRepo *repository.FeedbackRepository,
) *ConversationHandler {
	return &ConversationHandler{convoRepo: convoRepo, fbRepo: fbRepo}
}

// ---------------------------------------------------------------------------
// Conversations
// ---------------------------------------------------------------------------

// ListConversations returns paginated conversation logs for the caller.
func (h *ConversationHandler) ListConversations(c *gin.Context) {
	userID, _ := getUserID(c)
	page, size := parsePagination(c)
	logs, total, err := h.convoRepo.ListByUser(userID, page, size, c.Query("model"))
	if err != nil {
		response.InternalError(c, "failed to list conversations")
		return
	}
	models, _ := h.convoRepo.DistinctModels(userID)
	response.Success(c, gin.H{"items": logs, "total": total, "models": models})
}

// AdminListConversations returns paginated conversation logs across ALL users
// with optional filters (user_id / model / status). Mounted under /admin.
func (h *ConversationHandler) AdminListConversations(c *gin.Context) {
	page, size := parsePagination(c)
	modelName := c.Query("model")
	status := c.Query("status")
	userID := uint(0)
	if v := c.Query("user_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			userID = uint(id)
		}
	}
	logs, total, err := h.convoRepo.ListAll(page, size, userID, modelName, status)
	if err != nil {
		response.InternalError(c, "failed to list conversations")
		return
	}
	models, _ := h.convoRepo.DistinctModelsAll()
	response.Success(c, gin.H{"items": logs, "total": total, "models": models})
}

// AdminGetConversation returns a single conversation by id (any user).
func (h *ConversationHandler) AdminGetConversation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid conversation id")
		return
	}
	log, err := h.convoRepo.FindByID(uint(id))
	if err != nil || log == nil {
		response.NotFound(c, "conversation not found")
		return
	}
	response.Success(c, log)
}

// AdminExportConversations streams all (optionally filtered) conversations as JSONL.
func (h *ConversationHandler) AdminExportConversations(c *gin.Context) {
	modelName := c.Query("model")
	status := c.Query("status")
	userID := uint(0)
	if v := c.Query("user_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			userID = uint(id)
		}
	}
	logs, err := h.convoRepo.ExportAll(userID, modelName, status)
	if err != nil {
		response.InternalError(c, "failed to export conversations")
		return
	}

	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=conversations-all.jsonl")
	c.Header("Cache-Control", "no-store")

	w := c.Writer
	for _, log := range logs {
		var messages []map[string]interface{}
		if err := json.Unmarshal([]byte(log.Messages), &messages); err != nil || len(messages) == 0 {
			continue
		}
		var resp struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal([]byte(log.Response), &resp)
		row := make([]map[string]interface{}, 0, len(messages)+1)
		for _, m := range messages {
			row = append(row, m)
		}
		if resp.Content != "" {
			row = append(row, map[string]interface{}{"role": "assistant", "content": resp.Content})
		}
		line, err := json.Marshal(map[string]interface{}{"messages": row})
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "%s\n", line)
	}
	w.Flush()
}

// GetConversation returns a single conversation with full messages/response.
func (h *ConversationHandler) GetConversation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid conversation id")
		return
	}
	userID, _ := getUserID(c)
	log, err := h.convoRepo.FindByUserAndID(userID, uint(id))
	if err != nil || log == nil {
		response.NotFound(c, "conversation not found")
		return
	}
	response.Success(c, log)
}

// ExportConversations streams the caller's conversations as JSONL
// (OpenAI fine-tune style: one {"messages":[...]} object per line).
func (h *ConversationHandler) ExportConversations(c *gin.Context) {
	userID, _ := getUserID(c)
	logs, err := h.convoRepo.ListExport(userID)
	if err != nil {
		response.InternalError(c, "failed to export conversations")
		return
	}

	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=conversations.jsonl")
	c.Header("Cache-Control", "no-store")

	w := c.Writer
	for _, log := range logs {
		var messages []map[string]interface{}
		if err := json.Unmarshal([]byte(log.Messages), &messages); err != nil || len(messages) == 0 {
			continue
		}
		// Attach assistant reply to the conversation as the final message.
		var resp struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal([]byte(log.Response), &resp)
		row := make([]map[string]interface{}, 0, len(messages)+1)
		for _, m := range messages {
			row = append(row, m)
		}
		if resp.Content != "" {
			row = append(row, map[string]interface{}{"role": "assistant", "content": resp.Content})
		}
		line, err := json.Marshal(map[string]interface{}{"messages": row})
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "%s\n", line)
	}
	w.Flush()
}

// ---------------------------------------------------------------------------
// Feedback
// ---------------------------------------------------------------------------

type CreateFeedbackRequest struct {
	Type    string `json:"type" binding:"required,oneof=bug suggestion other"`
	Title   string `json:"title" binding:"required,max=200"`
	Content string `json:"content" binding:"required,max=10000"`
	Contact string `json:"contact" binding:"max=100"`
}

// CreateFeedback submits a program issue / suggestion report.
func (h *ConversationHandler) CreateFeedback(c *gin.Context) {
	var req CreateFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid feedback payload")
		return
	}
	userID, _ := getUserID(c)
	fb := &model.Feedback{
		UserID:  userID,
		Type:    req.Type,
		Title:   req.Title,
		Content: req.Content,
		Contact: req.Contact,
		Status:  "pending",
	}
	if err := h.fbRepo.Create(fb); err != nil {
		response.InternalError(c, "failed to submit feedback")
		return
	}
	response.Success(c, fb)
}

// ListMyFeedback returns the caller's submitted feedback.
func (h *ConversationHandler) ListMyFeedback(c *gin.Context) {
	page, size := parsePagination(c)
	userID, _ := getUserID(c)
	items, total, err := h.fbRepo.ListByUser(userID, page, size)
	if err != nil {
		response.InternalError(c, "failed to list feedback")
		return
	}
	response.Success(c, gin.H{"items": items, "total": total})
}

// GetFeedback returns one of the caller's feedback items.
func (h *ConversationHandler) GetFeedback(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid feedback id")
		return
	}
	userID, _ := getUserID(c)
	fb, err := h.fbRepo.FindByUserAndID(userID, uint(id))
	if err != nil || fb == nil {
		response.NotFound(c, "feedback not found")
		return
	}
	response.Success(c, fb)
}

// ---------------------------------------------------------------------------
// Admin feedback management
// ---------------------------------------------------------------------------

// AdminListFeedback returns all feedback with optional status filter.
func (h *ConversationHandler) AdminListFeedback(c *gin.Context) {
	page, size := parsePagination(c)
	items, total, err := h.fbRepo.ListAll(page, size, c.Query("status"))
	if err != nil {
		response.InternalError(c, "failed to list feedback")
		return
	}
	response.Success(c, gin.H{"items": items, "total": total})
}

type UpdateFeedbackStatusRequest struct {
	Status    string `json:"status"`
	AdminNote string `json:"admin_note" binding:"max=2000"`
}

// AdminUpdateFeedbackStatus updates feedback processing status and/or the
// admin note shown to the user.
func (h *ConversationHandler) AdminUpdateFeedbackStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid feedback id")
		return
	}
	var req UpdateFeedbackStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid payload")
		return
	}
	validStatus := map[string]bool{"pending": true, "processing": true, "resolved": true, "closed": true}
	changed := false
	if req.Status != "" {
		if !validStatus[req.Status] {
			response.BadRequest(c, "invalid status")
			return
		}
		if err := h.fbRepo.SetStatus(uint(id), req.Status); err != nil {
			response.InternalError(c, "failed to update feedback")
			return
		}
		changed = true
	}
	if req.AdminNote != "" {
		if err := h.fbRepo.SetNote(uint(id), req.AdminNote); err != nil {
			response.InternalError(c, "failed to update feedback")
			return
		}
		changed = true
	}
	if !changed {
		response.BadRequest(c, "nothing to update")
		return
	}
	response.SuccessWithMessage(c, "feedback updated", nil)
}
