package handlers

import (
	"log"
	"net/http"

	"github.com/s/onlineCourse/internal/models"
)

// Структура для отображения курса с процентами
type StudentCourseView struct {
	Enrollment      models.Enrollment
	ProgressPercent int
	TotalLessons    int
	DoneLessons     int
}

func (s *Handler) HandleStudentDashboard(w http.ResponseWriter, r *http.Request) {
	roleID, userID := s.GetUserRoleID(r)
	if userID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	session, _ := s.Store.Get(r, "session")
	var enrollments []models.Enrollment

	// Глубокий Preload: подтягиваем курс, автора курса, модули и уроки в модулях
	err := s.DB.Preload("Course.Author").
		Preload("Course.Modules.Lessons").
		Where("user_id = ?", userID).
		Find(&enrollments).Error

	if err != nil {
		log.Printf("Ошибка получения данных: %v", err)
	}

	// Подготовка данных для отображения (View)
	var views []StudentCourseView
	for _, e := range enrollments {
		// 1. Считаем общее количество уроков в курсе (проходим по модулям)
		totalLessons := 0
		for _, m := range e.Course.Modules {
			totalLessons += len(m.Lessons)
		}

		// 2. Считаем пройденные уроки из таблицы LessonProgress
		var doneLessons int64
		s.DB.Model(&models.LessonProgress{}).
			Where("user_id = ? AND course_id = ? AND is_done = ?", userID, e.CourseID, true).
			Count(&doneLessons)

		// 3. Считаем процент
		percent := 0
		if totalLessons > 0 {
			percent = int((float64(doneLessons) / float64(totalLessons)) * 100)
		}

		views = append(views, StudentCourseView{
			Enrollment:      e,
			ProgressPercent: percent,
			TotalLessons:    totalLessons,
			DoneLessons:     int(doneLessons),
		})
	}

	data := PageData{
		Title:           "Моё обучение",
		IsAuthenticated: true,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
		RoleID:          roleID,
		// ВАЖНО: передаем 'views', в которых есть и Enrollment, и посчитанный прогресс
		StudentCourses: views,
		CurrentPath:    r.URL.Path,
	}
	//w.Header().Set("Content-Type", "application/json")
	//fmt.Fprintf(w, "%+v", data)
	s.Tmpl.ExecuteTemplate(w, "studentDashboard", data)
}
