package i18n

import (
	"embed"
	"log/slog"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"botgo/internal/database"
)

//go:embed locales/*.yaml
var LocaleFS embed.FS

var (
	bundle *i18n.Bundle
	localizers = make(map[int64]*i18n.Localizer)
	userLangs = make(map[int64]string)
	mu sync.RWMutex
	initOnce sync.Once
)

func SupportedLanguages() []string {
	return []string{"ru", "en"}
}

var langLocalizers = make(map[string]*i18n.Localizer)

func TByLang(lang, key string) string {
	Init()

	mu.Lock()
	loc, ok := langLocalizers[lang]
	if !ok {
		loc = newLocalizer(lang)
		langLocalizers[lang] = loc
	}
	mu.Unlock()

	msg, err := loc.Localize(&i18n.LocalizeConfig{MessageID: key})
	if err != nil {
		return ""
	}
	return msg
}

// Init loads yaml locales. It is idempotent — repeated calls do nothing.
func Init() {
	initOnce.Do(func() {
		bundle = i18n.NewBundle(language.English)
		bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

		if _, err := bundle.LoadMessageFileFS(LocaleFS, "locales/active.ru.yaml"); err != nil {
			panic(err)
		}
		if _, err := bundle.LoadMessageFileFS(LocaleFS, "locales/active.en.yaml"); err != nil {
			panic(err)
		}
	})
}

// SetLanguage updates the language in the database and in the localizers cache.
func SetLanguage(userID int64, lang string) {
	Init()

	mu.Lock()
	defer mu.Unlock()

	if err := database.UpdateLanguage(userID, lang); err != nil {
		slog.Warn("Failed to update language in DB",
			"user_id", userID, "error", err)
	}

	localizers[userID] = newLocalizer(lang)
	userLangs[userID] = lang
}

// SetLanguageCacheOnly updates the language only in the cache,
// without writing to the database. It is used in handlers and tests.
func SetLanguageCacheOnly(userID int64, lang string) {
	Init()

	mu.Lock()
	defer mu.Unlock()

	localizers[userID] = newLocalizer(lang)
	userLangs[userID] = lang
}

// getOrCreateLocalizer returns the localizer from the cache or creates it.
// If the database is unavailable (for example, in tests where database.DB == nil),
// "en" is used as a fallback.
func getOrCreateLocalizer(userID int64) *i18n.Localizer {
	Init()

	mu.RLock()
	loc, ok := localizers[userID]
	mu.RUnlock()
	if ok {
		return loc
	}

	lang := "en"

	if database.DB != nil {
		if l, err := database.GetLanguage(userID); err == nil && l != "" {
			lang = l
		}
	}

	mu.Lock()
	defer mu.Unlock()

	// Re‑check after acquiring the write lock:
	// another goroutine might have filled the cache while we were waiting.
	if loc, ok := localizers[userID]; ok {
		return loc
	}

	loc = newLocalizer(lang)
	localizers[userID] = loc
	userLangs[userID] = lang
	return loc
}

// T returns the translation for the user.
func T(userID int64, key string, data ...map[string]interface{}) string {
	localizer := getOrCreateLocalizer(userID)

	var templateData map[string]interface{}
	if len(data) > 0 {
		templateData = data[0]
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: templateData,
	})
	if err != nil {
		slog.Warn("Translation missing",
			"user_id", userID,
			"key", key,
			"error", err,
		)
		return key
	}
	return msg
}

// newLocalizer creates a localizer for the language.
// Call only after Init() — otherwise bundle == nil.
func newLocalizer(lang string) *i18n.Localizer {
	var tag language.Tag
	switch lang {
	case "ru":
		tag = language.Russian
	default:
		tag = language.English
	}
	return i18n.NewLocalizer(bundle, tag.String())
}


func GetLanguage(userID int64) string {
	Init()

	mu.RLock()
	defer mu.RUnlock()

	if l, ok := userLangs[userID]; ok && l != "" {
		return l
	}
	return "en"
}