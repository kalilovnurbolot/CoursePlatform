package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/s/onlineCourse/internal/models"
	"gorm.io/gorm"
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
		StudentCourses:  views,
		CurrentPath:     r.URL.Path,
	}

	err = s.Tmpl.ExecuteTemplate(w, "studentDashboard", data)
	if err != nil {
		log.Printf("Ошибка рендеринга шаблона: %v", err)
		http.Error(w, "Ошибка сервера при формировании страницы", http.StatusInternalServerError)
	}
}

func (s *Handler) HandleCourseLearn(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	courseID := vars["id"]
	roleID, userID := s.GetUserRoleID(r)

	if userID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// 1. ПРОВЕРКА ДОСТУПА
	var enrollment models.Enrollment
	err := s.DB.Where("user_id = ? AND course_id = ? AND status = ?", userID, courseID, "approved").First(&enrollment).Error
	if err != nil {
		http.Error(w, "Доступ запрещен или заявка не одобрена", http.StatusForbidden)
		return
	}

	// 2. ЗАГРУЗКА ДАННЫХ КУРСА
	var course models.Course
	s.DB.Preload("Author").Preload("Modules.Lessons").First(&course, courseID)

	// 3. ЗАГРУЗКА ПРОГРЕССА
	var progress []models.LessonProgress
	s.DB.Where("user_id = ? AND course_id = ? AND is_done = ?", userID, courseID, true).Find(&progress)

	// Создаем карту пройденных уроков
	doneMap := make(map[uint]bool)
	for _, p := range progress {
		doneMap[p.LessonID] = true
	}

	// 4. РАСЧЕТЫ ДЛЯ ШАБЛОНА (Пагинация и Прогресс)
	totalLessons := 0
	var nextLessonID uint
	foundNext := false

	for _, m := range course.Modules {
		totalLessons += len(m.Lessons) // Считаем общее кол-во уроков

		for _, l := range m.Lessons {
			// Ищем первый урок, которого нет в карте выполненных
			if !doneMap[l.ID] && !foundNext {
				nextLessonID = l.ID
				foundNext = true
			}
		}
	}

	// Считаем процент
	percent := 0
	if totalLessons > 0 {
		percent = int((float64(len(doneMap)) / float64(totalLessons)) * 100)
	}

	session, _ := s.Store.Get(r, "session")

	// 5. ПОДГОТОВКА ДАННЫХ
	data := PageData{
		Title:           course.Title,
		Course:          course,
		IsAuthenticated: true,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
		DoneLessonsMap:  doneMap,
		CurrentPath:     r.URL.Path,
		RoleID:          roleID,
		TotalLessons:    totalLessons,
		ProgressPercent: percent,
		NextLessonID:    nextLessonID,
	}

	err = s.Tmpl.ExecuteTemplate(w, "courseContents", data)
	if err != nil {
		log.Printf("Ошибка рендеринга шаблона: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
	}
}

// HandleLessonView — Загрузка страницы урока
func (s *Handler) HandleLessonView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	courseID := vars["id"]
	lessonID := vars["lesson_id"]
	_, userID := s.GetUserRoleID(r)

	// 1. Загрузка урока и курса для навигации
	var lesson models.Lesson
	s.DB.Preload("ContentBlocks", func(db *gorm.DB) *gorm.DB {
		return db.Order("content_blocks.order ASC")
	}).First(&lesson, lessonID)

	var course models.Course
	s.DB.Preload("Modules.Lessons").First(&course, courseID)

	// 2. Логика поиска ID для кнопок "Назад" и "Вперед"
	var allLessons []uint
	for _, m := range course.Modules {
		for _, l := range m.Lessons {
			allLessons = append(allLessons, l.ID)
		}
	}

	var nextID, prevID uint
	for i, id := range allLessons {
		if id == lesson.ID {
			if i > 0 {
				prevID = allLessons[i-1]
			}
			if i < len(allLessons)-1 {
				nextID = allLessons[i+1]
			}
			break
		}
	}

	// 3. Загрузка прогресса и прошлых ответов
	var progress models.LessonProgress
	isDone := s.DB.Where("user_id = ? AND lesson_id = ? AND is_done = ?", userID, lesson.ID, true).First(&progress).RowsAffected > 0

	var attempts []models.QuizAttempt
	s.DB.Where("user_id = ? AND lesson_id = ?", userID, lessonID).Find(&attempts)
	attemptsJSON, _ := json.Marshal(attempts)
	attemptsStr := string(attemptsJSON)
	if attemptsStr == "null" || attemptsStr == "" {
		attemptsStr = "[]"
	}

	session, _ := s.Store.Get(r, "session")
	data := PageData{
		Title:           lesson.Title,
		Course:          course,
		Lesson:          lesson,
		IsAuthenticated: true,
		NextLessonID:    nextID,
		PrevLessonID:    prevID,
		IsLessonDone:    isDone,
		AttemptsJSON:    attemptsStr,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
		CourseLanguage:  course.Language,
	}
	s.Tmpl.ExecuteTemplate(w, "lessonView", data)
}

// SaveQuizAttemptAPI — Сохранение ответа СРАЗУ (POST /api/course/{id}/lesson/{lesson_id}/quiz)
func (s *Handler) SaveQuizAttemptAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	lessonID, _ := strconv.ParseUint(vars["lesson_id"], 10, 32)
	_, userID := s.GetUserRoleID(r)

	var req struct {
		BlockID       uint   `json:"block_id"`
		SelectedIndex int    `json:"selected_index"`
		Question      string `json:"question"`
		Answer        string `json:"answer"`
		IsCorrect     bool   `json:"is_correct"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Ошибка декодирования: %v", err)
		http.Error(w, "Error", http.StatusBadRequest)
		return
	}

	var block models.ContentBlock
	if err := s.DB.First(&block, req.BlockID).Error; err != nil {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}

	isCorrect := req.IsCorrect
	answerText := req.Answer

	// Проверка на сервере
	if block.Type == "quiz" {
		var quizData struct {
			CorrectIndex int      `json:"correct_index"`
			Options      []string `json:"options"`
		}
		if err := json.Unmarshal(block.Data, &quizData); err == nil {
			isCorrect = req.SelectedIndex == quizData.CorrectIndex
			if req.SelectedIndex >= 0 && req.SelectedIndex < len(quizData.Options) {
				answerText = quizData.Options[req.SelectedIndex]
			}
		}
	} else if block.Type == "audio_dictation" {
		var audioData struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(block.Data, &audioData); err == nil {
			isCorrect = req.Answer == audioData.Text
		}
	}

	attempt := models.QuizAttempt{
		UserID:        userID,
		LessonID:      uint(lessonID),
		BlockID:       req.BlockID,
		SelectedIndex: req.SelectedIndex,
		Question:      req.Question,
		Answer:        answerText,
		IsCorrect:     isCorrect,
	}

	s.DB.Where("user_id = ? AND block_id = ?", userID, req.BlockID).Delete(&models.QuizAttempt{})

	if err := s.DB.Create(&attempt).Error; err != nil {
		log.Printf("Ошибка записи в БД: %v", err)
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "saved",
		"is_correct": isCorrect,
	})
}

// MarkLessonReadAPI — Отметка о прочтении (POST /api/course/{id}/lesson/{lesson_id}/done)
func (s *Handler) MarkLessonReadAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	courseID, _ := strconv.ParseUint(vars["id"], 10, 32)
	lessonID, _ := strconv.ParseUint(vars["lesson_id"], 10, 32)
	_, userID := s.GetUserRoleID(r)

	s.DB.Where("user_id = ? AND lesson_id = ?", userID, lessonID).
		Assign(models.LessonProgress{IsDone: true, UpdatedAt: time.Now()}).
		FirstOrCreate(&models.LessonProgress{UserID: userID, LessonID: uint(lessonID), CourseID: uint(courseID)})
	w.WriteHeader(http.StatusOK)
}
