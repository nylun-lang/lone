package irtypes

type ImportKind int

const (
	IMP_KIND_CLASSIC ImportKind = iota
	IMP_KIND_SPECIFIC
)

type ImportStmt struct {
	Kind  ImportKind
	Value any
	Pos   Position
}

type ClassicImport struct {
	Path []string `json:"path"`
	Name string   `json:"alias"`
}

type SpecificImport struct {
	Path []string `json:"path"`
	Pick []ImportPick
}

type ImportPick struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
}
