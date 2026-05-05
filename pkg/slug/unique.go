package slug

import "fmt"

func (g *Generator) makeUnique(base string) string {
	if g.Checker == nil {
		return base
	}

	if !g.Checker.Exists(base) {
		return base
	}

	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)

		if !g.Checker.Exists(candidate) {
			return candidate
		}
	}
}