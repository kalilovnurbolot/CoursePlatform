# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

**Run with Docker (recommended):**
```bash
cp .env.example .env   # fill in your credentials
docker compose up --build
```

**Run locally with hot-reload (Air):**
```bash
go mod download
air
```

**Build manually:**
```bash
go build -o ./tmp/main ./cmd/main.go
./tmp/main
```

**Run DB migrations manually:** migrations run automatically on startup via `database.AutoMigrate()` — no manual step needed.

## Architecture

This is a Go web application using `gorilla/mux` for routing, `gorm` with PostgreSQL as the ORM/database, and `gorilla/sessions` for cookie-based sessions.

**Entry point:** `cmd/main.go` — wires up DB, migrations, seed, OAuth config, session store, all routes, and starts the HTTP server.

### Package layout

| Package | Role |
|---|---|
| `internal/handlers` | Base `Handler` struct — auth flows, main page, about page, language endpoint, `logAction` helper |
| `internal/handlers/cabinet.go` | `HandleCabinet` (personal dashboard), `HandleVerifyCertificate` |
| `internal/handlers/studio.go` | `HandleStudioPage` + all `/api/studio/...` course/module/lesson/content APIs (author-scoped) |
| `internal/handlers/userprofile.go` | `HandleUserProfilePage` — public profile at `/user/{public_id}` |
| `internal/handlers/admin` | Admin `Service` struct (embeds `Handler`) — course/module/lesson CRUD, enrollment management, journal/reports |
| `internal/handlers/feedback.go` | `AddCommentAPI`, `GetCommentsAPI` (lesson comments), `AddCourseCommentAPI`, `GetCourseCommentsAPI` (course comments), `AddReviewAPI`, `GetReviewsAPI` |
| `internal/handlers/reaction.go` | `ReactCourseAPI`, `GetCourseReactionsAPI`, `ReactLessonAPI`, `GetLessonReactionsAPI` — YouTube-style like/dislike toggle |
| `internal/handlers/admin/course_requests.go` | `HandleCourseRequestsPage`, `GetCourseRequestsAPI`, `ReviewCourseRequestAPI` — admin approval workflow |
| `internal/handlers/admin/journal.go` | `HandleJournalPage`, `GetJournalAPI` — paginated activity log viewer with action filter |
| `internal/handlers/admin/users_api.go` | `GetUsersAPI`, `UpdateUserRoleAPI` — user list with search/filter, role management, and per-user course count |
| `internal/handlers/personal` | Student "my courses" view |
| `internal/middleware` | `RequiredRole` — wraps `http.HandlerFunc`, checks session + DB role (`user.RoleID >= requiredRoleID`), renders 403 page on failure |
| `internal/models` | GORM models: `User` (has `PublicID` UUID for public URLs), `Role`, `Course`, `Module`, `Lesson`, `ContentBlock`, `Enrollment`, `LessonProgress`, `QuizAttempt`, `Comment`, `Review`, `Certificate`, `UserLog`, `Reaction` |
| `internal/auth` | Google OAuth2 config init |
| `internal/storage` | `SaveUser` — upserts a Google user into the DB; generates `PublicID` (UUID v4) on create, back-fills it for existing users |
| `internal/database` | `Connect` (5-retry loop), `AutoMigrate` (+ UUID back-fill + NOT NULL migration for `public_id`), `Seed` |
| `internal/i18n` | Translation loader and `T(lang, key)` lookup function |

### Authentication & roles

Authentication is Google OAuth only (`/auth/google/login` → `/auth/google/callback`). On callback, the user is upserted and their `user_id` is stored in the cookie session (`"session"` key).

Role constants are defined in `internal/models/role.go`:
- `RoleGuest = 0`, `RoleUser = 1`, `RoleAdmin = 2`, `RoleManager = 3`

`RequiredRole(h, models.RoleAdmin)` and `RequiredRole(h, models.RoleUser)` are used as route middleware wrappers in `main.go`. The check is **`user.RoleID >= requiredRoleID`** — higher roles always have access to lower-role routes.

### Data model hierarchy

```
Course → []Module → []Lesson → []ContentBlock
```

`ContentBlock.Type` can be: `"text"`, `"code"`, `"video"`, `"quiz"`, `"vocabulary"`, `"audio_dictation"`. `ContentBlock.Data` is a free-form `datatypes.JSON` field.

`Course` has two additional fields added for the creator workflow:
- `AdminStatus string` — `"draft" | "pending_review" | "approved" | "rejected"`. Default is `"approved"` (so existing and admin-created courses remain visible). Studio-created courses start as `"draft"`.
- `ReviewNote string` — filled by admin when rejecting, shown to the course author.

`Enrollment` links a `User` to a `Course` with a `Status` of `pending | approved | rejected`.

`LessonProgress` and `QuizAttempt` track per-user completion and quiz answers.

`Certificate` is auto-issued when a user completes 100% of a course (checked in `MarkLessonReadAPI`). Has a unique `(UserID, CourseID)` index — one cert per course. `Code` is a 32-char hex string for public verification at `/certificate/{code}`.

`Comment` has two optional FK fields: `LessonID` (lesson comment) and `CourseID` (course comment) — exactly one is non-zero.

`Reaction` stores like/dislike votes. Polymorphic: `TargetType ("course"|"lesson")` + `TargetID`. Unique index on `(UserID, TargetType, TargetID)` — one reaction per user per target. Toggle logic: same type → delete, different type → update.

`UserLog` records user activity. Action constants are in `internal/models/log.go`: `LogLogin`, `LogLessonView`, `LogQuizAttempt`, `LogCourseComplete`, `LogReviewAdded`, `LogCommentAdded`, `LogReactionAdded`. Written via `h.logAction(userID, action, details, courseID, lessonID)` helper on `Handler`.

### Templates

All HTML templates are parsed once at startup via `template.ParseGlob`. Templates in `template/*.html` and `template/**/*.html` share a single `*template.Template` instance with custom funcs (`mod`, `add`, `formatTime`, `T`). Template names match their filenames (e.g. `"index.html"`, `"cabinet.html"`).

Key templates added:

| Template | Route | Notes |
|---|---|---|
| `template/about.html` | `/about` | Public "About" page — mission, features, how-to guide, for-students/authors, CTA |
| `template/personal/studio.html` | `/studio` | Full course editor for regular users; calls `/api/studio/...` |
| `template/personal/user_profile.html` | `/user/{public_id}` | Public profile with course cards and enrollment actions |
| `template/admin/course_requests.html` | `/admin/course-requests` | Admin review panel; approve/reject with note |
| `template/admin/users.html` | `/admin/users` | Admin user management — search, filter by role, change role inline |
| `template/admin/journal.html` | `/admin/journal` | Activity journal — paginated table of `UserLog` rows, filter by action type, colour-coded badges |

`template/student/lessonView.html` includes a **reaction bar** (like/dislike) between lesson content and the comments section. Reactions load via `GET /api/lessons/{id}/reactions` and toggle via `POST /api/lessons/{id}/react`. Unauthenticated clicks redirect to login.

Use `{{ T .Lang "key" }}` in Go templates to get a translated string. For JavaScript inside templates, the full translation map is embedded via `{{.TransJSON}}` and accessed with `t('key')`:

```html
<script>const I18N = {{.TransJSON}};</script>
<script>function t(k){return I18N[k]||k;}</script>
```

### Open courses & free lessons

`Course.IsOpen` and `Lesson.IsFree` control public access. The routes `/course/{id}/learn` and `/course/{id}/lesson/{id}` intentionally do **not** use `userMiddleware` — access is checked inside the handler:

| Condition | Access |
|---|---|
| `course.IsOpen == true` | Anyone, no auth required |
| `lesson.IsFree == true` | Anyone for that lesson only |
| Neither | Requires auth + approved `Enrollment` |

Quiz/progress API endpoints (`/api/course/.../quiz`, `/api/course/.../done`) still require `userMiddleware` since saving progress requires knowing who the user is.

`GET /api/home` — server-side search, filter, sort, and pagination. Query params:

| Param | Values | Default |
|---|---|---|
| `search` | string | — |
| `filter` | `all` \| `open` \| `ru` \| `en` \| `ky` | `all` |
| `sort` | `newest` \| `az` \| `lessons` | `newest` |
| `page` | int | `1` |

Response: `{ courses, reviews, total, page, total_pages, counts: { all, open, ru, en, ky } }`. Page size is 12. `reviews` (top 6, rating ≥ 4) are only returned on the initial load (page 1, filter `all`, no search). `sort=lessons` uses a SQL subquery to count lessons per course server-side.

The home page (`template/index.html`) is a fully API-driven, Udemy-style layout:
- **Hero** — dark indigo gradient, large headline, search bar (300ms debounce → API call), popular-topic quick-search tags, decorative feature cards (desktop only)
- **Stats bar** — animated total course count (from `counts.all`) + free/cert/language highlights
- **Filter bar** (sticky `top-16`) — category pills (All / Open / RU / EN / KY) with server-returned counts, sort dropdown (newest / A–Z / most lessons)
- **Featured section** — top-4 courses from the current page; hidden when any filter/search/non-first-page is active
- **Course grid** — 4-column (xl), responsive down to 1; skeleton cards shown while loading; all filtering/sorting done server-side
- **Pagination** — numbered pages with Prev/Next; smart ellipsis (`1 … 4 5 6 … 10`); hidden when ≤1 page
- **Reviews** — horizontal scroll row, rendered once on first load, hidden if empty
- **Features** — 3-card section with CTA

JS state object: `{ search, filter, sort, page, totalPages }`. Any state change resets `page` to 1 and calls `loadCourses()` which builds query params and fetches `/api/home`.

### About page (`/about`)

`GET /about` — public, no auth required. Renders `about.html` via `HandleAboutPage`.

Sections (all text is i18n via `about.*` keys):
1. **Hero** — badge, title, subtitle
2. **Mission** — text block + 4 stat tiles (languages, courses, certificates, community)
3. **What you can do** — 4 feature cards: Learn / Create Courses / Get Certificates / Open Access
4. **How to get started** — 4 numbered steps (sign in → choose course → go through lessons → get certificate)
5. **For students / For authors** — two side-by-side checklist cards; CTA buttons adapt to auth state (authenticated → cabinet/studio, guest → Google login)
6. **Content types** — 6 block type icons: text, code, video, quiz, vocabulary, audio dictation
7. **CTA banner** — gradient section; authenticated users see "My Account", guests see Google login button

Nav link in `template/layouts/header.html` points to `/about` (was `#about`).

### Personal cabinet

`GET /cabinet` — main personal dashboard, requires `userMiddleware`. Renders `cabinet.html`.

`HandleCabinet` calls `buildCabinetData(userID)` which aggregates in one pass:
- **Stats**: enrolled count, completed (= certificate count), lessons done, quiz accuracy %
- **Courses**: split into `InProgress` / `Completed` (has certificate) / `Pending` (pending|rejected enrollments)
- **AuthoredCourses**: courses where `author_id = userID`, with student count and avg rating
- **Activity**: last 10 `UserLog` rows
- **Reviews**: user's own reviews with preloaded `Course`
- **Certificates**: user's certificates with preloaded `Course`

`GET /certificate/{code}` — public verification page, no auth required. Renders `certificate.html`.

`GET /personal` — redirects 301 to `/cabinet` (kept for backward compatibility).

### User Studio (`/studio`)

`GET /studio` — requires `userMiddleware`. Any authenticated user can create and manage their own courses. Renders `studio.html`.

The studio page is **full-width** (no `max-w` constraint) and fully responsive. It shows all courses where `author_id = userID` (all statuses).

**Course cards display:**
- `admin_status` badge (overlaid on the cover image)
- Rejection note if status is `rejected`
- Module count and lesson count (from preloaded `Modules.Lessons`)
- `created_at` and `updated_at` dates formatted with `toLocaleDateString`
- Context-appropriate action buttons (Edit content / Settings / Submit / Delete)

**Editor layout:**
- Desktop (≥1024 px): 3-column grid — Structure tree (300 px) | Block editor (flex) | Lesson settings (300 px)
- Mobile (<1024 px): tab bar at the top switches between Structure / Editor / Settings panels; selecting a lesson auto-switches to the Editor tab

**Studio API endpoints** — all require `userMiddleware`; each handler additionally checks `author_id = userID`:

| Method | Path | Action |
|---|---|---|
| `GET` | `/api/studio/courses` | List own courses — preloads `Author` + `Modules.Lessons` for card counts |
| `POST` | `/api/studio/courses` | Create course (sets `admin_status = "draft"`) |
| `PUT` | `/api/studio/courses/{id}` | Update basic info (blocked if `pending_review`) |
| `DELETE` | `/api/studio/courses/{id}` | Delete (blocked if `approved`) |
| `POST` | `/api/studio/courses/{id}/submit` | Submit for review → `pending_review` (requires ≥1 lesson) |
| `GET` | `/api/studio/courses/{id}/structure` | Full module/lesson tree |
| `POST/PUT/DELETE` | `/api/studio/modules/{id}` | Module CRUD (checks course author) |
| `POST/GET/PUT/DELETE` | `/api/studio/lessons/{id}` | Lesson CRUD (checks course author) |
| `PUT` | `/api/studio/lessons/{id}/content` | Save content blocks (same 409/force_reset logic as admin API) |

**Editing rules:** A course in `pending_review` cannot be edited. Editing a `rejected` course resets it to `draft` and clears `review_note`.

### Course admin-status lifecycle

```
[User creates]
    draft
      │  user clicks "Submit for review"
      ▼
  pending_review
      │  admin approves          admin rejects (with note)
      ▼                              ▼
  approved                        rejected
  (is_published=true → visible)     │  user edits → resets to draft
                                     └──► draft → ...
```

Admin actions are at `GET /admin/course-requests` (page) and `PUT /api/admin/course-requests/{id}` with `{"action":"approve"|"reject","review_note":"..."}`.

### Reactions (like/dislike)

Polymorphic `Reaction` model — one table for both courses and lessons.

**API endpoints:**

| Method | Path | Auth | Action |
|---|---|---|---|
| `POST` | `/api/courses/{id}/react` | user | Toggle like/dislike on course |
| `GET` | `/api/courses/{id}/reactions` | — | Get counts + caller's current reaction |
| `POST` | `/api/lessons/{id}/react` | user | Toggle like/dislike on lesson |
| `GET` | `/api/lessons/{id}/reactions` | — | Get counts + caller's current reaction |

Request body: `{"type": "like"|"dislike"}`. Response: `{"likes": N, "dislikes": N, "user_reaction": "like"|"dislike"|""}`.

Every POST writes a `LogReactionAdded` entry to `user_logs`.

### Comments

Lesson comments: `POST/GET /api/lessons/{id}/comments` (existing).  
Course comments: `POST/GET /api/courses/{id}/comments` (new).

Both POST handlers write a `LogCommentAdded` entry to `user_logs`.

### Admin activity journal (`/admin/journal`)

`GET /admin/journal` — requires `adminMiddleware`. Renders `adminJournal` template.

`GET /api/admin/journal` — paginated log of `user_logs`, 20 per page.

| Param | Values | Default |
|---|---|---|
| `action` | `all` \| `login` \| `lesson_view` \| `comment_added` \| `reaction_added` \| `quiz_attempt` \| `course_complete` \| `review_added` | `all` |
| `page` | int | `1` |

Response: `{logs, total, page, total_pages}`. Each log row preloads `User`. The template renders colour-coded action badges per type and smart-ellipsis pagination.

### Admin user management (`/admin/users`)

`GET /admin/users` — requires `adminMiddleware`. Renders `users.html` (template name `adminUsers`).

**API endpoints** — both require `adminMiddleware`:

| Method | Path | Action |
|---|---|---|
| `GET` | `/api/admin/users` | Paginated user list; query params: `search` (name/email ILIKE), `role` (RoleID int), `page` (default 1, 20 per page). Returns `id`, `public_id`, `name`, `email`, `picture`, `role_id`, `role`, `course_count` |
| `PUT` | `/api/admin/users/{id}/role` | Change a user's role; body `{"role_id": uint}` |

**Business rules:**
- An admin cannot demote their own role below `RoleAdmin` (returns `403 cannot demote yourself`).
- `role_id` must match an existing `Role` row or the request is rejected with `400`.
- The page renders a table with avatar, name (links to `/user/{public_id}`), email, colour-coded role badge, course count (badge links to profile), and an inline `<select>` for instant role change.

### Public user profile (`/user/{public_id}`)

`GET /user/{public_id}` — public, no auth required. Renders `user_profile.html`. `public_id` is a UUID v4 stored on `User.PublicID`; numeric IDs are never exposed in URLs to prevent enumeration.

Shows the profiled user's avatar, name, and all their courses where `admin_status = 'approved' AND is_published = true`. For the current viewer, enrollment status per course is looked up so the correct action button is shown (Start / Go to course / Pending / Enroll / Login).

`ProfileCourseView` (defined in `userprofile.go`, also registered in `PageData`):
```go
type ProfileCourseView struct {
    Course           models.Course
    LessonCount      int
    EnrollmentStatus string // "pending" | "approved" | "rejected" | ""
}
```

### Key invariant: content block updates

`PUT /api/lessons/{id}/content` uses a transaction that checks whether any `QuizAttempt` rows reference blocks being modified or deleted. If they do and `force_reset` is false, it returns `409 BLOCK_HAS_ANSWERS`. Callers must pass `"force_reset": true` to override.

### Internationalisation (i18n)

The app supports three languages: **Russian (`ru`)**, **English (`en`)**, **Kyrgyz (`ky`)**.

**Translation files:** `locales/ru.json`, `locales/en.json`, `locales/ky.json` — flat key/value JSON, ~520 keys each.  
**Loader:** `i18n.Load("locales")` is called once in `main.go` before anything else.

**Language detection priority** (highest → lowest):
1. `lang` cookie (set by `/api/language` or header language switcher)
2. `User.Language` field in DB (restored to cookie on login)
3. `Accept-Language` HTTP header
4. Default `ru`

**`PageData` fields every handler must set:**
```go
lang := h.DetectLang(r)          // method on Handler
Lang:      lang,
TransJSON: BuildTransJSON(lang),  // exported helper in handlers package
```

**Language change flow:**
- `POST /api/language` `{"lang": "en"}` — sets `lang` cookie (1 year) and, if authenticated, updates `User.Language` in DB.
- On Google login callback, the pre-login cookie lang is persisted to DB; if no cookie, the saved DB lang is restored to cookie.
- First-visit JS modal: shown when `navigator.language` base code ≠ `SITE_LANG` and `localStorage.lang_chosen` is not set.

**Key namespaces added:**
- `about.*` — About page strings: mission, feature cards, how-to steps, for-students/authors lists, CTA
- `studio.*` — User Studio page strings, including:
  - `studio.created` / `studio.updated` — date labels on course cards
  - `studio.modules` / `studio.lessons` — unit suffixes for counts on cards
  - `studio.tab_structure` / `studio.tab_editor` / `studio.tab_settings` — mobile editor tab labels
- `userprofile.*` — Public user profile page strings
- `creq.*` — Admin course-requests panel strings
- `nav.studio` — "My Studio" navigation link
- `admin.users_*` — Admin user management page strings
- `admin.journal_*` — Admin activity journal page strings (title, action labels, filter options, badges)
- `reaction.*` — Like/dislike button labels and login prompt
- `comment.course_*` — Course-level comment section strings
- `home.page_prev` / `home.page_next` — Pagination Prev/Next button labels

**Adding a new translation key:**
1. Add the key/value to all three `locales/*.json` files.
2. Use `{{ T .Lang "your.key" }}` in Go templates or `t('your.key')` in JS.

**`User.Language`** — `string` column, default `'ru'`, updated via `/api/language`.

## Environment variables

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | PostgreSQL DSN |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` / `GOOGLE_REDIRECT_URL` | OAuth2 — all three are required at startup |
| `SESSION_KEY` | Cookie signing key (falls back to hardcoded default if unset) |
| `PORT` | HTTP listen port (default `8080`) |
| `APP_PORT` | External port in docker-compose |
