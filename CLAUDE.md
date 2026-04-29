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
| `internal/handlers` | Base `Handler` struct (holds DB, session store, OAuth config, parsed templates), auth flows, main page, profile, language endpoint |
| `internal/handlers/admin` | Admin `Service` struct (embeds `Handler`) — course/module/lesson CRUD API, enrollment management, journal/reports pages |
| `internal/handlers/personal` | Student "my courses" view |
| `internal/middleware` | `RequiredRole` — wraps `http.HandlerFunc`, checks session + DB role, renders 403 page on failure |
| `internal/models` | GORM models: `User`, `Role`, `Course`, `Module`, `Lesson`, `ContentBlock`, `Enrollment`, `LessonProgress`, `QuizAttempt`, `Comment`, `Review` |
| `internal/auth` | Google OAuth2 config init |
| `internal/storage` | `SaveUser` — upserts a Google user into the DB |
| `internal/database` | `Connect` (5-retry loop), `AutoMigrate`, `Seed` |
| `internal/i18n` | Translation loader and `T(lang, key)` lookup function |

### Authentication & roles

Authentication is Google OAuth only (`/auth/google/login` → `/auth/google/callback`). On callback, the user is upserted and their `user_id` is stored in the cookie session (`"session"` key).

Role constants are defined in `internal/models/role.go`:
- `RoleGuest = 0`, `RoleUser = 1`, `RoleAdmin = 2`, `RoleManager = 3`

`RequiredRole(h, models.RoleAdmin)` and `RequiredRole(h, models.RoleUser)` are used as route middleware wrappers in `main.go`.

### Data model hierarchy

```
Course → []Module → []Lesson → []ContentBlock
```

`ContentBlock.Type` can be: `"text"`, `"code"`, `"video"`, `"quiz"`, `"vocabulary"`, `"audio_dictation"`. `ContentBlock.Data` is a free-form `datatypes.JSON` field.

`Enrollment` links a `User` to a `Course` with a `Status` of `pending | approved | rejected`.

`LessonProgress` and `QuizAttempt` track per-user completion and quiz answers.

### Templates

All HTML templates are parsed once at startup via `template.ParseGlob`. Templates in `template/*.html` and `template/**/*.html` share a single `*template.Template` instance with custom funcs (`mod`, `add`, `formatTime`, `T`). Template names match their filenames (e.g. `"index.html"`, `"profile.html"`).

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

`/api/home` returns `open_courses []Course` (separate from `courses`) — only courses where `is_published = true AND is_open = true`, preloaded with `Modules.Lessons` for the lesson count on the card.

### Key invariant: content block updates

`PUT /api/lessons/{id}/content` uses a transaction that checks whether any `QuizAttempt` rows reference blocks being modified or deleted. If they do and `force_reset` is false, it returns `409 BLOCK_HAS_ANSWERS`. Callers must pass `"force_reset": true` to override.

### Internationalisation (i18n)

The app supports three languages: **Russian (`ru`)**, **English (`en`)**, **Kyrgyz (`ky`)**.

**Translation files:** `locales/ru.json`, `locales/en.json`, `locales/ky.json` — flat key/value JSON, ~150 keys each.  
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
