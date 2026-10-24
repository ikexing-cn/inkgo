// Command inkgo is the native writing application.
//
//	go run ./cmd/inkgo
//	go run ./cmd/inkgo -work <work-directory> -trace <request-log-directory>
package main

import (
	"flag"
	"log"
	"os"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"writer/ai"
	"writer/desk"
)

func main() {
	workPath := flag.String("work", "", "打开作品文件夹")
	trace := flag.String("trace", "", "把每次请求的全文写进这个文件夹（调提示词用，不写进作品）")
	flag.Parse()
	// The AI keeps its services in this machine's settings folder, and
	// the shelf of works lies beside them.
	path, err := ai.ConfigPath()
	service := ai.New(path)
	service.Trace = *trace
	d := desk.New(service)
	if err != nil {
		d.SetError(err)
	}
	d.Backups = true
	// The dialogs open over the window, asked for off the UI thread.
	var win atomic.Pointer[mygo.Window]
	d.PickFolder = func(title string) (string, error) {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win.Load(), Title: title, Directory: true, CreateDirectories: true})
		if err != nil || len(paths) == 0 {
			return "", err
		}
		return paths[0], nil
	}
	d.SaveFile = func(title, name string) (string, error) {
		return mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: win.Load(), Title: title, DefaultPath: name, CreateDirectories: true,
			Filters: []mygo.FileFilter{{Name: "纯文本", Extensions: []string{"txt"}}}})
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
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "Inkgo", Width: 1280, Height: 820, MinWidth: 1000, MinHeight: 680, StateKey: "main",
			// The page draws its own title bar, a row over everything, as the design does.
			// The window buttons fill that row, as high as desk's titleHeight.
			TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 40, Content: ui.View(d.View)})
		win.Store(w)
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
