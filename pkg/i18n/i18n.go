// Package i18n provides internationalization for GoliveKit.
package i18n

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Translator provides translation functionality.
type Translator struct {
	locale       string
	translations map[string]map[string]string // locale -> key -> value
	fallback     string
	pluralRules  map[string]PluralRule
	mu           sync.RWMutex
}

// PluralRule defines how to pluralize for a locale.
type PluralRule func(count int) string

// NewTranslator creates a new translator.
func NewTranslator(defaultLocale string) *Translator {
	return &Translator{
		locale:       defaultLocale,
		translations: make(map[string]map[string]string),
		fallback:     defaultLocale,
		pluralRules:  defaultPluralRules(),
	}
}

// SetLocale sets the current locale.
func (t *Translator) SetLocale(locale string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.locale = locale
}

// Locale returns the current locale.
func (t *Translator) Locale() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.locale
}

// SetFallback sets the fallback locale.
func (t *Translator) SetFallback(locale string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fallback = locale
}

// Load loads translations for a locale.
func (t *Translator) Load(locale string, translations map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.translations[locale] == nil {
		t.translations[locale] = make(map[string]string)
	}

	for key, value := range translations {
		t.translations[locale][key] = value
	}
}

// T translates a key to the current locale.
func (t *Translator) T(key string, args ...any) string {
	t.mu.RLock()
	locale := t.locale
	fallback := t.fallback
	t.mu.RUnlock()

	// Try current locale
	if value := t.get(locale, key); value != "" {
		return t.interpolate(value, args...)
	}

	// Try fallback
	if locale != fallback {
		if value := t.get(fallback, key); value != "" {
			return t.interpolate(value, args...)
		}
	}

	// Return key as is
	return key
}

// TPlural translates with pluralization.
func (t *Translator) TPlural(key string, count int, args ...any) string {
	t.mu.RLock()
	locale := t.locale
	t.mu.RUnlock()

	// Get plural form
	pluralKey := t.getPluralKey(locale, key, count)

	// Try to find translation
	if value := t.get(locale, pluralKey); value != "" {
		return t.interpolate(value, append([]any{count}, args...)...)
	}

	// Fall back to singular
	return t.T(key, append([]any{count}, args...)...)
}

// TLocale translates for a specific locale.
func (t *Translator) TLocale(locale, key string, args ...any) string {
	if value := t.get(locale, key); value != "" {
		return t.interpolate(value, args...)
	}

	t.mu.RLock()
	fallback := t.fallback
	t.mu.RUnlock()

	if locale != fallback {
		if value := t.get(fallback, key); value != "" {
			return t.interpolate(value, args...)
		}
	}

	return key
}

func (t *Translator) get(locale, key string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if translations, ok := t.translations[locale]; ok {
		return translations[key]
	}
	return ""
}

func (t *Translator) interpolate(template string, args ...any) string {
	if len(args) == 0 {
		return template
	}

	// Support both positional and named arguments
	result := template

	// Handle positional: %1, %2, etc.
	for i, arg := range args {
		placeholder := fmt.Sprintf("%%%d", i+1)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprint(arg))
	}

	// Handle map arguments for named placeholders
	if len(args) == 1 {
		if m, ok := args[0].(map[string]any); ok {
			for key, value := range m {
				placeholder := fmt.Sprintf("{{%s}}", key)
				result = strings.ReplaceAll(result, placeholder, fmt.Sprint(value))
			}
		}
	}

	return result
}

func (t *Translator) getPluralKey(locale, key string, count int) string {
	t.mu.RLock()
	rule := t.pluralRules[locale]
	t.mu.RUnlock()

	if rule == nil {
		rule = defaultPluralRules()["en"]
	}

	form := rule(count)
	return key + "." + form
}

// Default plural rules for common languages
func defaultPluralRules() map[string]PluralRule {
	return map[string]PluralRule{
		"en": func(count int) string {
			if count == 1 {
				return "one"
			}
			return "other"
		},
		"es": func(count int) string {
			if count == 1 {
				return "one"
			}
			return "other"
		},
		"fr": func(count int) string {
			if count <= 1 {
				return "one"
			}
			return "other"
		},
		"de": func(count int) string {
			if count == 1 {
				return "one"
			}
			return "other"
		},
		"ru": func(count int) string {
			mod10 := count % 10
			mod100 := count % 100
			if mod10 == 1 && mod100 != 11 {
				return "one"
			}
			if mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20) {
				return "few"
			}
			return "many"
		},
		"zh": func(count int) string {
			return "other"
		},
		"ja": func(count int) string {
			return "other"
		},
		"ko": func(count int) string {
			return "other"
		},
		"ar": func(count int) string {
			if count == 0 {
				return "zero"
			}
			if count == 1 {
				return "one"
			}
			if count == 2 {
				return "two"
			}
			mod100 := count % 100
			if mod100 >= 3 && mod100 <= 10 {
				return "few"
			}
			if mod100 >= 11 {
				return "many"
			}
			return "other"
		},
	}
}

// Context helpers

type i18nContextKey struct{}

// WithTranslator adds a translator to context.
func WithTranslator(ctx context.Context, t *Translator) context.Context {
	return context.WithValue(ctx, i18nContextKey{}, t)
}

// TranslatorFromContext retrieves a translator from context.
func TranslatorFromContext(ctx context.Context) *Translator {
	t, _ := ctx.Value(i18nContextKey{}).(*Translator)
	return t
}

// T translates using the translator from context.
func T(ctx context.Context, key string, args ...any) string {
	t := TranslatorFromContext(ctx)
	if t == nil {
		return key
	}
	return t.T(key, args...)
}

// TPlural translates with pluralization using context translator.
func TPlural(ctx context.Context, key string, count int, args ...any) string {
	t := TranslatorFromContext(ctx)
	if t == nil {
		return key
	}
	return t.TPlural(key, count, args...)
}

// LocaleFromContext extracts locale from context.
func LocaleFromContext(ctx context.Context) string {
	t := TranslatorFromContext(ctx)
	if t == nil {
		return "en"
	}
	return t.Locale()
}

// Bundle manages multiple translators for different locales.
type Bundle struct {
	translators map[string]*Translator
	defaultLoc  string
	mu          sync.RWMutex
}

// NewBundle creates a new translation bundle.
func NewBundle(defaultLocale string) *Bundle {
	return &Bundle{
		translators: make(map[string]*Translator),
		defaultLoc:  defaultLocale,
	}
}

// AddTranslations adds translations for a locale.
func (b *Bundle) AddTranslations(locale string, translations map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.translators[locale] == nil {
		b.translators[locale] = NewTranslator(locale)
	}
	b.translators[locale].Load(locale, translations)
}

// Translator returns a translator for a locale.
func (b *Bundle) Translator(locale string) *Translator {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if t, ok := b.translators[locale]; ok {
		return t
	}

	// Fall back to default
	if t, ok := b.translators[b.defaultLoc]; ok {
		return t
	}

	return nil
}

// Locales returns all available locales.
func (b *Bundle) Locales() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	locales := make([]string, 0, len(b.translators))
	for locale := range b.translators {
		locales = append(locales, locale)
	}
	return locales
}
