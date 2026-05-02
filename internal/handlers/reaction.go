package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/s/onlineCourse/internal/models"
)

type reactionCountsResponse struct {
	Likes        int64  `json:"likes"`
	Dislikes     int64  `json:"dislikes"`
	UserReaction string `json:"user_reaction"` // "like" | "dislike" | ""
}

func (h *Handler) reactionCounts(targetType string, targetID uint, userID uint) reactionCountsResponse {
	var likes, dislikes int64
	h.DB.Model(&models.Reaction{}).Where("target_type = ? AND target_id = ? AND type = ?", targetType, targetID, "like").Count(&likes)
	h.DB.Model(&models.Reaction{}).Where("target_type = ? AND target_id = ? AND type = ?", targetType, targetID, "dislike").Count(&dislikes)

	userReaction := ""
	if userID > 0 {
		var existing models.Reaction
		if h.DB.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).First(&existing).Error == nil {
			userReaction = existing.Type
		}
	}
	return reactionCountsResponse{Likes: likes, Dislikes: dislikes, UserReaction: userReaction}
}

func (h *Handler) handleReact(w http.ResponseWriter, r *http.Request, targetType string, targetID uint, courseID uint, lessonID uint) {
	_, userID := h.GetUserRoleID(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Type != "like" && req.Type != "dislike") {
		http.Error(w, "type must be 'like' or 'dislike'", http.StatusBadRequest)
		return
	}

	var existing models.Reaction
	result := h.DB.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).First(&existing)

	if result.Error == nil {
		if existing.Type == req.Type {
			// Same type → toggle off (delete)
			h.DB.Delete(&existing)
		} else {
			// Different type → update
			h.DB.Model(&existing).Update("type", req.Type)
		}
	} else {
		// No existing → create
		h.DB.Create(&models.Reaction{
			UserID:     userID,
			TargetType: targetType,
			TargetID:   targetID,
			Type:       req.Type,
		})
	}

	h.logAction(userID, models.LogReactionAdded,
		fmt.Sprintf("%s %s #%d", req.Type, targetType, targetID),
		courseID, lessonID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.reactionCounts(targetType, targetID, userID))
}

// POST /api/courses/{id}/react
func (h *Handler) ReactCourseAPI(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	h.handleReact(w, r, "course", uint(id), uint(id), 0)
}

// GET /api/courses/{id}/reactions
func (h *Handler) GetCourseReactionsAPI(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	_, userID := h.GetUserRoleID(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.reactionCounts("course", uint(id), userID))
}

// POST /api/lessons/{id}/react
func (h *Handler) ReactLessonAPI(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	h.handleReact(w, r, "lesson", uint(id), 0, uint(id))
}

// GET /api/lessons/{id}/reactions
func (h *Handler) GetLessonReactionsAPI(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	_, userID := h.GetUserRoleID(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.reactionCounts("lesson", uint(id), userID))
}
