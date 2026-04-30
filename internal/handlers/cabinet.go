package handlers

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/s/onlineCourse/internal/i18n"
	"github.com/s/onlineCourse/internal/models"
)

type CabinetStats struct {
	EnrolledCount  int
	CompletedCount int // = количество сертификатов
	LessonsDone    int
	QuizAccuracy   int // 0-100
	CertCount      int
}

type CabinetCourseView struct {
	Enrollment      models.Enrollment
	ProgressPercent int
	TotalLessons    int
	DoneLessons     int
}

type AuthoredCourseView struct {
	Course       models.Course
	StudentCount int64
	AvgRating    float64
}

type CabinetData struct {
	Stats           CabinetStats
	InProgress      []CabinetCourseView
	Completed       []CabinetCourseView
	Pending         []models.Enrollment
	AuthoredCourses []AuthoredCourseView
	Activity        []models.UserLog
	Reviews         []models.Review
	Certificates    []models.Certificate
}

func (h *Handler) HandleCabinet(w http.ResponseWriter, r *http.Request) {
	roleID, userID := h.GetUserRoleID(r)
	if userID == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	session, _ := h.Store.Get(r, "session")
	lang := h.DetectLang(r)

	data := PageData{
		Title:           i18n.T(lang, "cabinet.title"),
		IsAuthenticated: true,
		UserID:          userID,
		RoleID:          roleID,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
		Email:           toString(session.Values["email"]),
		CurrentPath:     r.URL.Path,
		Lang:            lang,
		TransJSON:       BuildTransJSON(lang),
		Cabinet:         h.buildCabinetData(userID),
	}

	if err := h.Tmpl.ExecuteTemplate(w, "cabinet.html", data); err != nil {
		log.Printf("HandleCabinet: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleVerifyCertificate — публичная страница верификации сертификата по коду
func (h *Handler) HandleVerifyCertificate(w http.ResponseWriter, r *http.Request) {
	code := mux.Vars(r)["code"]

	var cert models.Certificate
	if err := h.DB.Preload("User").Preload("Course").
		Where("code = ?", code).First(&cert).Error; err != nil {
		http.Error(w, "Сертификат не найден", http.StatusNotFound)
		return
	}

	lang := h.DetectLang(r)
	_, userID := h.GetUserRoleID(r)
	session, _ := h.Store.Get(r, "session")

	data := PageData{
		Title:           i18n.T(lang, "cert.verify_title"),
		IsAuthenticated: userID != 0,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
		Lang:            lang,
		TransJSON:       BuildTransJSON(lang),
		Certificate:     cert,
	}

	if err := h.Tmpl.ExecuteTemplate(w, "certificate.html", data); err != nil {
		log.Printf("HandleVerifyCertificate: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) buildCabinetData(userID uint) CabinetData {
	var d CabinetData

	// --- СТАТИСТИКА ---
	var enrolledCount int64
	h.DB.Model(&models.Enrollment{}).Where("user_id = ?", userID).Count(&enrolledCount)

	var lessonsDone int64
	h.DB.Model(&models.LessonProgress{}).
		Where("user_id = ? AND is_done = ?", userID, true).Count(&lessonsDone)

	var certCount int64
	h.DB.Model(&models.Certificate{}).Where("user_id = ?", userID).Count(&certCount)

	var totalAttempts, correctAttempts int64
	h.DB.Model(&models.QuizAttempt{}).Where("user_id = ?", userID).Count(&totalAttempts)
	h.DB.Model(&models.QuizAttempt{}).
		Where("user_id = ? AND is_correct = ?", userID, true).Count(&correctAttempts)
	accuracy := 0
	if totalAttempts > 0 {
		accuracy = int(float64(correctAttempts) / float64(totalAttempts) * 100)
	}

	d.Stats = CabinetStats{
		EnrolledCount:  int(enrolledCount),
		CompletedCount: int(certCount),
		LessonsDone:    int(lessonsDone),
		QuizAccuracy:   accuracy,
		CertCount:      int(certCount),
	}

	// --- КУРСЫ: ЗАПИСИ ---
	var enrollments []models.Enrollment
	h.DB.Preload("Course.Author").
		Preload("Course.Modules.Lessons").
		Where("user_id = ?", userID).
		Find(&enrollments)

	// Карта выданных сертификатов — разделяем завершённые / в процессе
	var certs []models.Certificate
	h.DB.Where("user_id = ?", userID).Find(&certs)
	certMap := make(map[uint]bool, len(certs))
	for _, c := range certs {
		certMap[c.CourseID] = true
	}

	for _, e := range enrollments {
		if e.Status == "pending" || e.Status == "rejected" {
			d.Pending = append(d.Pending, e)
			continue
		}

		totalLessons := 0
		for _, m := range e.Course.Modules {
			totalLessons += len(m.Lessons)
		}

		var doneLessons int64
		h.DB.Model(&models.LessonProgress{}).
			Where("user_id = ? AND course_id = ? AND is_done = ?", userID, e.CourseID, true).
			Count(&doneLessons)

		percent := 0
		if totalLessons > 0 {
			percent = int(float64(doneLessons) / float64(totalLessons) * 100)
		}

		view := CabinetCourseView{
			Enrollment:      e,
			ProgressPercent: percent,
			TotalLessons:    totalLessons,
			DoneLessons:     int(doneLessons),
		}

		if certMap[e.CourseID] {
			d.Completed = append(d.Completed, view)
		} else {
			d.InProgress = append(d.InProgress, view)
		}
	}

	// --- АВТОРСКИЕ КУРСЫ ---
	var authored []models.Course
	h.DB.Preload("Author").Where("author_id = ?", userID).Find(&authored)

	for _, c := range authored {
		var studentCount int64
		h.DB.Model(&models.Enrollment{}).
			Where("course_id = ? AND status = ?", c.ID, "approved").
			Count(&studentCount)

		var avgRating float64
		h.DB.Model(&models.Review{}).
			Select("COALESCE(AVG(rating), 0)").
			Where("course_id = ?", c.ID).
			Scan(&avgRating)

		d.AuthoredCourses = append(d.AuthoredCourses, AuthoredCourseView{
			Course:       c,
			StudentCount: studentCount,
			AvgRating:    avgRating,
		})
	}

	// --- АКТИВНОСТЬ (последние 10) ---
	h.DB.Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(10).
		Find(&d.Activity)

	// --- МОИ ОТЗЫВЫ ---
	h.DB.Preload("Course").
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&d.Reviews)

	// --- СЕРТИФИКАТЫ ---
	h.DB.Preload("Course").
		Where("user_id = ?", userID).
		Order("issued_at desc").
		Find(&d.Certificates)

	return d
}
