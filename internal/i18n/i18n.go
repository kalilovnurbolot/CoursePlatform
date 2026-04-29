package i18n

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

const DefaultLang = "ru"

var supportedLangs = map[string]bool{
	"ru": true,
	"en": true,
	"ky": true,
}

var (
	store = map[string]map[string]string{}
	mu    sync.RWMutex
)

func Load(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	for lang := range supportedLangs {
		data, err := os.ReadFile(dir + "/" + lang + ".json")
		if err != nil {
			return err
		}
		var msgs map[string]string
		if err := json.Unmarshal(data, &msgs); err != nil {
			return err
		}
		store[lang] = msgs
	}
	return nil
}

// T returns the translation for the given key in the given language,
// falling back to DefaultLang, then the key itself.
func T(lang, key string) string {
	mu.RLock()
	defer mu.RUnlock()

	if msgs, ok := store[lang]; ok {
		if v, ok := msgs[key]; ok {
			return v
		}
	}
	if msgs, ok := store[DefaultLang]; ok {
		if v, ok := msgs[key]; ok {
			return v
		}
	}
	return key
}

// GetAll returns a copy of all translations for the given language.
func GetAll(lang string) map[string]string {
	mu.RLock()
	defer mu.RUnlock()

	src := store[lang]
	if src == nil {
		src = store[DefaultLang]
	}
	cp := make(map[string]string, len(src))
	for k, v := range src {
		cp[k] = v
	}
	return cp
}

// IsSupported reports whether lang is a known language code.
func IsSupported(lang string) bool {
	return supportedLangs[lang]
}

// FromAcceptLanguage parses an Accept-Language header value and returns
// the best supported language code, or DefaultLang if none matches.
func FromAcceptLanguage(header string) string {
	for _, part := range strings.Split(header, ",") {
		code := strings.TrimSpace(strings.Split(part, ";")[0])
		base := strings.ToLower(strings.Split(code, "-")[0])
		if supportedLangs[base] {
			return base
		}
	}
	return DefaultLang
}
