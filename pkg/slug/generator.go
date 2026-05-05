package slug

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func New(checker SlugChecker) *Generator {
	return &Generator{
		MaxInputRunes: 2000,
		MaxSlugLen:    80,
		Checker:       checker,
	}
}

func (g *Generator) Generate(s string) string {
	s = g.sanitizeInput(s)

	slug := g.slugify(s)
	slug = g.normalize(slug)

	if g.Checker != nil {
		slug = g.makeUnique(slug)
	}

	return slug
}

func (g *Generator) slugify(s string) string {
	s = strings.ToLower(s)

	var b strings.Builder
	b.Grow(len(s))

	prevDash := false

	for _, r := range s {
		if val, ok := translit[r]; ok {
			if val != "" {
				b.WriteString(val)
				prevDash = false
			}
			continue
		}

		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}

		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}

	return strings.Trim(b.String(), "-")
}

func (g *Generator) sanitizeInput(s string) string {
	if g.MaxInputRunes <= 0 {
		return s
	}

	if utf8.RuneCountInString(s) <= g.MaxInputRunes {
		return s
	}

	r := []rune(s)
	return string(r[:g.MaxInputRunes])
}