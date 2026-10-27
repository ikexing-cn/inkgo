package shelf

import "math"

// ZoomSteps are the toolkit's content scales from 50% to 200%.
func ZoomSteps() []float32 {
	return []float32{0.5, 0.67, 0.75, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2}
}

// NearestZoom snaps z to a step; zero is actual size.
func NearestZoom(z float32) float32 {
	if z == 0 {
		return 1
	}
	steps := ZoomSteps()
	best := steps[0]
	for _, step := range steps[1:] {
		if math.Abs(float64(z-step)) < math.Abs(float64(z-best)) {
			best = step
		}
	}
	return best
}
