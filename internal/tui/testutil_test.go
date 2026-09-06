package tui

func absRatio(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
