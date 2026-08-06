package types

type ModuleDec struct {
	Name string `json:"name"`
}

type ImportDec struct {
	Path  string `json:"path"`
	Alias string `json:"alias"`
}

type Program struct {
	Mod  ModuleDec
	Imp  []ImportDec
	Body []any
}
