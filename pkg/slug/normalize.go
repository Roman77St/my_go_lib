package slug

import "strings"

var stopWords = map[string]struct{}{
	"i": {}, "v": {}, "vo": {}, "na": {}, "s": {}, "so": {},
	"o": {}, "k": {}, "u": {}, "po": {},
}

func (g *Generator) normalize(slug string) string {
	if slug == "" {
		return ""
	}

	parts := strings.Split(slug, "-")

	// 1. убираем стоп-слова в конце
	for len(parts) > 0 {
		last := parts[len(parts)-1]
		if _, ok := stopWords[last]; ok {
			parts = parts[:len(parts)-1]
		} else {
			break
		}
	}

	// 2. ограничение длины по словам
	if g.MaxSlugLen > 0 {
		var out []string
		length := 0

		for _, p := range parts {
			addLen := len(p)
			if len(out) > 0 {
				addLen++ // дефис
			}

			if length+addLen > g.MaxSlugLen {
				break
			}

			out = append(out, p)
			length += addLen
		}

		parts = out
	}

	result := strings.Join(parts, "-")
	return strings.Trim(result, "-")
}