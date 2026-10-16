// Poc is a proof of concept of the novel editor in pure Go: a native
// window drawn by mygo, with Chinese text, input methods, marks, entity
// completion and inline predictions (from a stand-in, not a model).
//
//	go run ./cmd/poc
package main

import (
	"flag"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"writer/ai"
	"writer/desk"
	"writer/editor"
)

const sample = "　　第一章　雨夜\n" +
	"　　雨下了整整一夜。林晚站在檐下，看着院子里那口老井，井沿上的青苔被雨水洗得发亮。\n" +
	"　　“你还记得那把钥匙吗？”沈砚从屋里走出来，手里提着一盏旧灯。\n" +
	"　　她没有回答。那把铜钥匙一直挂在她的脖子上，贴着心口，凉得像一块冰。\n" +
	"　　Tab 接受灰色预测，Esc 拒绝；输入“林”或“沈”试试人名补全；选中文字后右键可标记为伏笔。"

type app struct {
	ed         *editor.Editor
	lastEditor ui.Rect // the editor's bounds, for tests
}

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Fill().Children(func() {
		// A row centers its children across by default: both fill the
		// window's height.
		el := a.ed.View(c).Grow(1).FillHeight().Background(t.Background)
		a.lastEditor = el.Bounds()
		ui.Column(c).Width(240).FillHeight().Padding(16).Gap(10).Background(t.Surface).Children(func() {
			ui.Text(c, "状态").Bold()
			a0, b0 := a.ed.Selection()
			ui.Textf(c, "字数 %d　光标 %d　选区 %d", a.ed.Doc.Len(), a.ed.Caret(), b0-a0).FontSize(12).TextColor(t.TextMuted)
			switch {
			case a.ed.Preedit() != "":
				ui.Textf(c, "输入法：%s", a.ed.Preedit()).FontSize(12)
			case a.ed.Predicting():
				ui.Text(c, "预测中…").FontSize(12).TextColor(t.TextMuted)
			case a.ed.Ghost() != "":
				ui.Text(c, "Tab 接受预测").FontSize(12).TextColor(t.Accent)
			}
			ui.Divider(c)
			ui.Text(c, "标记").Bold()
			for _, m := range a.ed.Doc.Marks() {
				s := ""
				if m.End > m.Start {
					s = a.ed.Doc.Slice(m.Start, min(m.End, m.Start+12))
				}
				tag := m.Kind.String()
				if m.Orphaned {
					tag += "（原文已删）"
				}
				ui.Textf(c, "%s [%d,%d) %s", tag, m.Start, m.End, s).FontSize(12)
			}
			ui.Spacer(c)
			if ui.Button(c, "标记选区为伏笔").Clicked() {
				a.ed.MarkSelection(editor.MarkForeshadow, "")
				a.ed.Focus()
			}
		})
	})
}

func main() {
	editorDemo := flag.Bool("editor", false, "打开原有正文编辑器演示")
	workPath := flag.String("work", "", "打开作品文件夹")
	flag.Parse()
	if !*editorDemo {
		// The AI keeps its services in this machine's settings folder, and
		// the shelf of works lies beside them.
		path, err := ai.ConfigPath()
		d := desk.New(ai.New(path))
		if err != nil {
			d.Error = err.Error()
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
				d.Error = err.Error()
			}
		} else {
			d.OpenLast()
		}
		defer d.Close()
		// The appearance chosen on this machine, before the window shows.
		mygo.Theme.SetSource(mygo.ThemeSource(d.Theme()))
		d.Appearance = func(v string) { mygo.Theme.SetSource(mygo.ThemeSource(v)) }
		mygo.App.WhenReady(func() {
			win.Store(mygo.NewWindow(mygo.WindowOptions{Title: "Inkgo", Width: 1280, Height: 820, MinWidth: 1000, MinHeight: 680, StateKey: "main",
				// The page draws its own title bar, a row over everything, as the design does.
				// The window buttons fill that row, as high as desk's titleHeight.
				TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 40, Content: ui.View(d.View)}))
		})
		if err := mygo.App.Run(); err != nil {
			log.Fatal(err)
		}
		return
	}
	ents := []editor.Entity{
		{Name: "林晚", Kind: "人物"}, {Name: "林晚舟", Kind: "人物"},
		{Name: "沈砚", Kind: "人物"}, {Name: "青石镇", Kind: "地名"},
	}
	a := &app{ed: editor.New(editor.NewDoc(sample), editor.Options{
		Entities:     ents,
		Predictor:    editor.DemoPredictor{Delay: 600 * time.Millisecond},
		PredictDelay: 800 * time.Millisecond,
	})}
	a.ed.MoveTo(a.ed.Doc.Len(), false)
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title: "写作 PoC", Width: 1100, Height: 720, MinWidth: 600, MinHeight: 400,
			Content: ui.View(a.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(fmt.Errorf("run: %w", err))
	}
}
