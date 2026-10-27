package desk

import "writer/shelf"

// Zoom is the content scale chosen on this machine; zero means actual size.
func (a *App) Zoom() float32 {
	if a.shelf.Zoom == 0 {
		return 1
	}
	return a.shelf.Zoom
}

// zoomNext is the next step in dir, or zero at the end or beyond the screen.
func (a *App) zoomNext(dir int) float32 {
	steps := shelf.ZoomSteps()
	if dir > 0 {
		for _, z := range steps {
			if z > a.Zoom() && (a.ZoomFits == nil || a.ZoomFits(z)) {
				return z
			}
		}
	} else {
		for i := len(steps) - 1; i >= 0; i-- {
			if steps[i] < a.Zoom() {
				return steps[i]
			}
		}
	}
	return 0
}

// zoomBy changes the whole interface, leaving the prose size alone.
func (a *App) zoomBy(dir int) {
	z := float32(1)
	if dir != 0 {
		z = a.zoomNext(dir)
	}
	if z == 0 || z == a.Zoom() {
		return
	}
	a.shelf.Zoom = z
	if z == 1 {
		a.shelf.Zoom = 0
	}
	a.saveShelf()
	if a.ZoomChanged != nil {
		a.ZoomChanged(z)
	}
}
