package slug

type SlugChecker interface {
	Exists(slug string) bool
}

type Generator struct {
	MaxInputRunes int
	MaxSlugLen    int
	Checker       SlugChecker
}