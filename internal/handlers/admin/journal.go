package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/s/onlineCourse/internal/handlers"
	"github.com/s/onlineCourse/internal/i18n"
	"github.com/s/onlineCourse/internal/models"
)

func (serv Service) HandleJournalPage(w http.ResponseWriter, r *http.Request) {
	roleID, userID := serv.GetUserRoleID(r)
	session, _ := serv.Store.Get(r, "session")
	lang := serv.DetectLang(r)

	name, _ := session.Values["name"].(string)
	picture, _ := session.Values["picture"].(string)

	data := handlers.PageData{
		Title:           i18n.T(lang, "admin.journal_title"),
		IsAuthenticated: userID != 0,
		UserID:          userID,
		RoleID:          roleID,
		UserName:        name,
		UserPictureURL:  picture,
		CurrentPath:     r.URL.Path,
		Lang:            lang,
		TransJSON:       handlers.BuildTransJSON(lang),
	}

	serv.Tmpl.ExecuteTemplate(w, "adminJournal", data)
}

// GET /api/admin/journal?action=all&page=1
func (serv Service) GetJournalAPI(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const pageSize = 20
	offset := (page - 1) * pageSize

	query := serv.DB.Model(&models.UserLog{}).Preload("User")
	if action != "" && action != "all" {
		query = query.Where("action = ?", action)
	}

	var total int64
	query.Count(&total)

	var logs []models.UserLog
	query.Order("created_at desc").Limit(pageSize).Offset(offset).Find(&logs)

	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":        logs,
		"total":       total,
		"page":        page,
		"total_pages": totalPages,
	})
}
