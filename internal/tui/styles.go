package tui

type Styles struct {
	Primary  string
	TabLabel string
}

func NewStyles() Styles {
	return Styles{
		Primary:  "#fab283",
		TabLabel: "tab",
	}
}
