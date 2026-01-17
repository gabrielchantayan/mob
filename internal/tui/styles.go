package tui

type Styles struct {
	Primary string
}

func NewStyles() Styles {
	return Styles{
		Primary: "#fab283",
	}
}
