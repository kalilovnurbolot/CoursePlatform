package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/s/onlineCourse/internal/models"
)

func (serv Service) GetUsersAPI(w http.ResponseWriter, r *http.Request) {
	session, err := serv.Store.Get(r, "session")
	if err != nil {
		http.Error(w, `{"error":"session error"}`, http.StatusInternalServerError)
		return
	}
	callerID, _ := session.Values["user_id"].(uint)
	if callerID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	search := r.URL.Query().Get("search")
	roleFilter := r.URL.Query().Get("role")
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	const perPage = 20

	query := serv.DB.Model(&models.User{}).Preload("Role")
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("name ILIKE ? OR email ILIKE ?", like, like)
	}
	if roleFilter != "" {
		rid, _ := strconv.Atoi(roleFilter)
		query = query.Where("role_id = ?", rid)
	}

	var total int64
	query.Count(&total)

	var users []models.User
	query.Offset((page-1)*perPage).Limit(perPage).Order("id asc").Find(&users)

	// batch-count courses per author
	userIDs := make([]uint, 0, len(users))
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}
	var counts []struct {
		AuthorID uint
		Count    int64
	}
	serv.DB.Model(&models.Course{}).
		Select("author_id, count(*) as count").
		Where("author_id IN ?", userIDs).
		Group("author_id").
		Scan(&counts)
	courseCount := make(map[uint]int64, len(counts))
	for _, c := range counts {
		courseCount[c.AuthorID] = c.Count
	}

	type UserRow struct {
		ID          uint   `json:"id"`
		PublicID    string `json:"public_id"`
		Name        string `json:"name"`
		Email       string `json:"email"`
		Picture     string `json:"picture"`
		RoleID      uint   `json:"role_id"`
		Role        string `json:"role"`
		CourseCount int64  `json:"course_count"`
	}
	rows := make([]UserRow, 0, len(users))
	for _, u := range users {
		roleName := ""
		if u.Role.ID != 0 {
			roleName = u.Role.Name
		}
		rows = append(rows, UserRow{
			ID:          u.ID,
			PublicID:    u.PublicID,
			Name:        u.Name,
			Email:       u.Email,
			Picture:     u.Picture,
			RoleID:      u.RoleID,
			Role:        roleName,
			CourseCount: courseCount[u.ID],
		})
	}

	pages := int(total) / perPage
	if int(total)%perPage != 0 {
		pages++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  rows,
		"total": total,
		"page":  page,
		"pages": pages,
	})
}

func (serv Service) UpdateUserRoleAPI(w http.ResponseWriter, r *http.Request) {
	session, err := serv.Store.Get(r, "session")
	if err != nil {
		http.Error(w, `{"error":"session error"}`, http.StatusInternalServerError)
		return
	}
	callerID, _ := session.Values["user_id"].(uint)
	if callerID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	targetID, err := strconv.ParseUint(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	var body struct {
		RoleID uint `json:"role_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	var role models.Role
	if err := serv.DB.First(&role, body.RoleID).Error; err != nil {
		http.Error(w, `{"error":"role not found"}`, http.StatusBadRequest)
		return
	}

	if uint(targetID) == callerID && body.RoleID < models.RoleAdmin {
		http.Error(w, `{"error":"cannot demote yourself"}`, http.StatusForbidden)
		return
	}

	if err := serv.DB.Model(&models.User{}).Where("id = ?", targetID).Update("role_id", body.RoleID).Error; err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}