package tui

type Chooser struct {
	Options []string
	Index   int
}

func NewChooser(options []string) *Chooser {
	return &Chooser{Options: options}
}

func (c *Chooser) Next() {
	if len(c.Options) == 0 {
		return
	}
	c.Index = (c.Index + 1) % len(c.Options)
}

func clampHeight(height int) int {
	if height < 3 {
		return 3
	}
	if height > 24 {
		return 24
	}
	return height
}
