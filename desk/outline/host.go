package outline

import "writer/ext/hostapi"

// Services contains host contracts used by the outline on the frame thread.
type Services struct {
	// Book supplies current work content.
	Book hostapi.Book
	// Palette supplies frame colors.
	Palette hostapi.Palette
	// Notices reports storage failures.
	Notices hostapi.Notices
	// Dialogs supplies native file callbacks.
	Dialogs hostapi.Dialogs
	// Desktop supplies navigation and shared frame widgets.
	Desktop hostapi.Desktop
}
