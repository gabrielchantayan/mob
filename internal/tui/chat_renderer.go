package tui

func clampHeight(height int) int {
	if height < 3 {
		return 3
	}
	if height > 24 {
		return 24
	}
	return height
}
