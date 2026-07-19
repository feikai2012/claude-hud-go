// Package i18n ports src/i18n/*: message catalogs for en / zh-Hans / zh-Hant
// with zh and zh-TW aliases, plus {placeholder} interpolation.
package i18n

import (
	"regexp"
	"strconv"
)

type canonical string

const (
	canEn     canonical = "en"
	canZhHans canonical = "zh-Hans"
	canZhHant canonical = "zh-Hant"
)

var canonicalMap = map[string]canonical{
	"en":      canEn,
	"zh":      canZhHans,
	"zh-Hans": canZhHans,
	"zh-Hant": canZhHant,
	"zh-TW":   canZhHant,
}

var locales = map[canonical]map[string]string{
	canEn:     en,
	canZhHans: zhHans,
	canZhHant: zhHant,
}

var current = canEn

// SetLanguage sets the active language (accepts any of the aliases).
func SetLanguage(lang string) {
	if c, ok := canonicalMap[lang]; ok {
		current = c
		return
	}
	current = canEn
}

// IsCjkLanguage reports whether the active language is a CJK locale.
func IsCjkLanguage() bool {
	return current == canZhHans || current == canZhHant
}

// T looks up a message key, falling back to English then the key itself.
func T(key string) string {
	if v, ok := locales[current][key]; ok {
		return v
	}
	if v, ok := en[key]; ok {
		return v
	}
	return key
}

var placeholder = regexp.MustCompile(`\{(\w+)\}`)

// Interpolate replaces {name} placeholders; unknown placeholders become "".
func Interpolate(pattern string, params map[string]any) string {
	return placeholder.ReplaceAllStringFunc(pattern, func(m string) string {
		key := m[1 : len(m)-1]
		v, ok := params[key]
		if !ok {
			return ""
		}
		switch x := v.(type) {
		case string:
			return x
		case int:
			return strconv.Itoa(x)
		case float64:
			return strconv.FormatFloat(x, 'g', -1, 64)
		default:
			return ""
		}
	})
}
