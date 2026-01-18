package i18n

import (
	"embed"
	"log"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed *.yaml
var translationFiles embed.FS

type Language string

const (
	LangUA Language = "ua"
	LangEN Language = "en"
)

func (l Language) String() string {
	return string(l)
}

func (l Language) IsValid() bool {
	return l == LangUA || l == LangEN
}

func ParseLanguage(s string) Language {
	switch strings.ToLower(s) {
	case "en", "eng", "english":
		return LangEN
	default:
		return LangUA
	}
}

type Translator struct {
	translations map[Language]map[string]string
}

func NewTranslator() *Translator {
	t := &Translator{
		translations: make(map[Language]map[string]string),
	}
	t.loadTranslations()
	return t
}

func (t *Translator) loadTranslations() {
	languages := []struct {
		lang Language
		file string
	}{
		{LangUA, "ua.yaml"},
		{LangEN, "en.yaml"},
	}

	for _, l := range languages {
		data, err := translationFiles.ReadFile(l.file)
		if err != nil {
			log.Printf("Failed to load translation file %s: %v", l.file, err)
			continue
		}

		var translations map[string]string
		if err := yaml.Unmarshal(data, &translations); err != nil {
			log.Printf("Failed to parse translation file %s: %v", l.file, err)
			continue
		}

		t.translations[l.lang] = translations
	}
}

func (t *Translator) Get(key string, lang Language) string {
	if translations, ok := t.translations[lang]; ok {
		if text, ok := translations[key]; ok {
			return text
		}
	}

	// Fallback to Ukrainian
	if translations, ok := t.translations[LangUA]; ok {
		if text, ok := translations[key]; ok {
			return text
		}
	}

	return key
}

func (t *Translator) GetWithParams(key string, lang Language, params map[string]string) string {
	text := t.Get(key, lang)
	for k, v := range params {
		text = strings.ReplaceAll(text, "{"+k+"}", v)
	}
	return text
}

func (t *Translator) GetLanguageName(lang Language) string {
	return t.Get("language_name", lang)
}
