// Command inkgo is the native writing application.
//
//	go run ./cmd/inkgo
//	go run ./cmd/inkgo -work <work-directory> -trace <request-log-directory>
package main

import (
	"flag"
	"log"
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"writer/ai"
	"writer/ai/aitext"
	"writer/desk"
	"writer/locales"
)

func main() {
	if runtime.GOOS == "darwin" {
		mygo.App.SetName("Inkgo")
	}
	workPath := flag.String("work", "", "Open a book folder")
	trace := flag.String("trace", "", "Write full request traces to this folder for prompt debugging (outside the book)")
	flag.Parse()
	// The AI keeps its services in this machine's settings folder, and
	// the shelf of works lies beside them.
	path, err := ai.ConfigPath()
	service := ai.New(path)
	service.Trace = *trace
	d := desk.New(service, modules...)
	mygo.App.SetLanguages(locales.Catalog.Languages()...)
	// The in-app choice is authoritative: on macOS this intentionally
	// replaces a per-app language changed in System Settings at next launch.
	// MYGO_LOCALE still overrides this startup call without being saved.
	mygo.App.SetLocale(d.Locale())
	d.SetLocale = mygo.App.SetLocale
	mygo.App.OnLocaleChanged(func(string) { d.LocaleChanged() })
	if err != nil {
		d.SetError(err)
	}
	// The dialogs open over the window, asked for off the UI thread.
	var win atomic.Pointer[mygo.Window]
	var area atomic.Pointer[mygo.Rectangle]
	d.ZoomFits = func(z float32) bool {
		return zoomFits(area.Load(), z)
	}
	var wanted atomic.Uint32
	wanted.Store(math.Float32bits(d.Zoom()))
	var resizing sync.Mutex
	d.ZoomChanged = func(z float32) {
		wanted.Store(math.Float32bits(z))
		go func() {
			resizing.Lock()
			defer resizing.Unlock()
			if w := win.Load(); w != nil {
				resizeForZoom(w, math.Float32frombits(wanted.Load()))
			}
		}()
	}
	d.PickFolder = func(title string) (string, error) {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win.Load(), Title: title, Directory: true, CreateDirectories: true})
		if err != nil || len(paths) == 0 {
			return "", err
		}
		return paths[0], nil
	}
	d.SaveFile = func(title, name string) (string, error) {
		return mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: win.Load(), Title: title, DefaultPath: name, CreateDirectories: true,
			Filters: []mygo.FileFilter{{Name: locales.ExportPlain(), Extensions: []string{"txt"}}}})
	}
	d.Reveal = desk.RevealFolder
	d.SetTitle = func(title string) bool {
		w := win.Load()
		if w != nil {
			w.SetTitle(title)
		}
		return w != nil
	}
	if *workPath != "" {
		if err := d.Open(*workPath); err != nil {
			d.SetError(err)
		}
	} else {
		d.OpenLast()
	}
	// The work closes once the window has, and before the software opens
	// again when it relaunches: the one that opens finds it saved.
	mygo.App.OnQuit(func() { d.Close() })
	// The appearance chosen on this machine, before the window shows.
	mygo.Theme.SetSource(mygo.ThemeSource(d.Theme()))
	d.Appearance = func(v string) { mygo.Theme.SetSource(mygo.ThemeSource(v)) }
	// Drawn without the GPU where this machine says so: mygo looks as the
	// window draws its first frame, so a change waits for the next start.
	if d.NoGPU() {
		os.Setenv("MYGO_GPU", "0")
	}
	d.Relaunch = mygo.App.Relaunch
	d.ErrorText = []desk.ErrorTranslator{aitext.Translate}
	mygo.App.WhenReady(func() {
		installSystemMenu(d, func() {
			if w := win.Load(); w != nil {
				w.Invalidate()
			}
		})
		zoom := math.Float32frombits(wanted.Load())
		minWidth, minHeight := zoomMinimum(zoom)
		w := mygo.NewWindow(mygo.WindowOptions{Title: "Inkgo", Width: max(windowWidth, minWidth), Height: max(windowHeight, minHeight), MinWidth: minWidth, MinHeight: minHeight, StateKey: "main",
			// The page draws its own title bar, a row over everything, as the design does.
			// The window buttons fill that row, as high as desk's titleHeight.
			TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: windowTitleHeight, Content: ui.View(d.View)})
		win.Store(w)
		updateActivity := func() {
			d.SetWindowActive(w.IsFocused() && !w.IsMinimized())
			w.Invalidate()
		}
		w.OnFocus(updateActivity)
		w.OnBlur(updateActivity)
		w.OnMinimize(updateActivity)
		w.OnRestore(updateActivity)
		updateActivity()
		// Window bounds and display work areas use DIPs, including on Windows.
		// The view reads this cache without waiting for the window thread.
		updateArea := func() {
			work := mygo.Screen.DisplayMatching(w.Bounds()).WorkArea
			area.Store(&work)
			w.Invalidate()
		}
		updateArea()
		w.OnMove(updateArea)
		w.OnResize(updateArea)
		off := mygo.Screen.OnDisplaysChanged(updateArea)
		w.OnClosed(off)
		// Restored window bounds may be smaller than the saved zoom requires.
		resizeForZoom(w, zoom)
		// Closed with what the AI proposed not decided on, the window asks
		// first; let go, it closes after the frame that let it go.
		w.OnClose(func(e *mygo.CloseEvent) {
			if d.Leave(func() { go w.Close() }) {
				e.PreventDefault()
				w.Invalidate()
			}
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
