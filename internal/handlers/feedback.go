package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/s/onlineCourse/internal/models"
)

// --- КОММЕНТАРИИ ---

// POST /api/lessons/{id}/comments
func (h *Handler) AddCommentAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	lessonID, _ := strconv.Atoi(vars["id"])
	_, userID := h.GetUserRoleID(r)

	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "Content is required", http.StatusBadRequest)
		return
	}

	comment := models.Comment{
		UserID:   userID,
		LessonID: uint(lessonID),
		Content:  req.Content,
	}

	if err := h.DB.Create(&comment).Error; err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Подгружаем пользователя для ответа
	h.DB.Preload("User").First(&comment, comment.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comment)
}

// GET /api/lessons/{id}/comments
func (h *Handler) GetCommentsAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	lessonID, _ := strconv.Atoi(vars["id"])

	var comments []models.Comment
	// Загружаем комментарии с данными пользователей, сортируем от новых к старым
	if err := h.DB.Preload("User").Where("lesson_id = ?", lessonID).Order("created_at desc").Find(&comments).Error; err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

// --- ОТЗЫВЫ ---

// POST /api/courses/{id}/reviews
func (h *Handler) AddReviewAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	courseID, _ := strconv.Atoi(vars["id"])
	_, userID := h.GetUserRoleID(r)

	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Rating  int    `json:"rating"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Rating < 1 || req.Rating > 5 {
		http.Error(w, "Rating must be between 1 and 5", http.StatusBadRequest)
		return
	}

	// Проверяем, есть ли уже отзыв от этого пользователя
	var review models.Review
	result := h.DB.Where("user_id = ? AND course_id = ?", userID, courseID).First(&review)

	if result.RowsAffected > 0 {
		// Обновляем существующий
		review.Rating = req.Rating
		review.Content = req.Content
		h.DB.Save(&review)
	} else {
		// Создаем новый
		review = models.Review{
			UserID:   userID,
			CourseID: uint(courseID),
			Rating:   req.Rating,
			Content:  req.Content,
		}
		h.DB.Create(&review)
	}

	h.DB.Preload("User").First(&review, review.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(review)
}

// GET /api/courses/{id}/reviews
func (h *Handler) GetReviewsAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	courseID, _ := strconv.Atoi(vars["id"])

	var reviews []models.Review
	if err := h.DB.Preload("User").Where("course_id = ?", courseID).Order("created_at desc").Find(&reviews).Error; err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reviews)
}
