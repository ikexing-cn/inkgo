package desk

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/desk/kit"
	"writer/shelf"
	"writer/workspace"
)

// Library is where a first shelf looks for works and new works go; tests
// point it somewhere of their own.
var Library = shelf.DefaultLibrary

// shelfView is the state of the shelf and its dialogs.
type shelfView struct {
	file    string
	broken  bool // works.json did not read: it is never written over
	missing map[string]bool
	// The new-work dialog: where it goes, and whether the author chose it
	// rather than following the name; what is there, as last looked at.
	creating bool
	at, auto string
	looked   string
	place    string
	fits     bool
	// The work being renamed and its new name.
	renaming, name string
}

// loadShelf reads the shelf beside the model configuration, starting one
// from the library the first time.
func (a *App) loadShelf() {
	a.books.file = shelf.File(a.ai.ConfigFile())
	s, fresh, err := shelf.Load(a.books.file)
	if err != nil {
		a.books.broken = true
		a.SetError(fmt.Errorf("作品清单无法读取，本次不会改动：%w", err))
		return
	}
	if fresh {
		s = shelf.Start(Library())
		for i, b := range s.Books {
			if d, err := shelf.Describe(b.Path); err == nil {
				s.Books[i] = d
			}
		}
	}
	a.shelf = s
	if fresh {
		a.saveShelf()
	}
	a.lookForBooks()
}

func (a *App) saveShelf() {
	if a.books.broken || a.books.file == "" {
		return
	}
	a.warn(a.shelf.Save(a.books.file))
}

// lookForBooks notes the works whose folders are gone.
func (a *App) lookForBooks() {
	a.books.missing = map[string]bool{}
	for _, b := range a.shelf.Books {
		if !workspace.IsWork(b.Path) {
			a.books.missing[b.Path] = true
		}
	}
}

// remember puts the open work on the shelf as it is now.
func (a *App) remember(opened bool) {
	if a.Store == nil {
		return
	}
	b := shelf.DescribeOpen(a.Store)
	if opened {
		b.Opened = time.Now()
	}
	a.shelf.Put(b)
	a.saveShelf()
	delete(a.books.missing, b.Path)
}

// OpenLast opens the work last opened, if its folder is still there.
func (a *App) OpenLast() {
	recent := a.shelf.Recent()
	if len(recent) == 0 || recent[0].Opened.IsZero() || !workspace.IsWork(recent[0].Path) {
		return
	}
	a.SetError(a.Open(recent[0].Path))
}

// switchWork opens another work from the shelf.
func (a *App) switchWork(path string) {
	if !workspace.IsWork(path) {
		a.books.missing[path] = true
		a.SetError(errors.New("找不到该作品的文件夹：" + path))
		return
	}
	a.SetError(a.Open(path))
}

// openCreate starts the new-work dialog.
func (a *App) openCreate() {
	a.name, a.genre = "", "网文"
	a.books.creating, a.books.at, a.books.auto, a.books.place = true, "", "", ""
}

// placeFor says what is at the folder a new work would go in, and whether
// the work can go there: a folder not made yet, or an empty one. It names
// the folder, which a long path in the field above leaves out.
func placeFor(dir string) (say string, fits bool) {
	if dir == "" {
		return "请填写作品位置。", false
	}
	name := "「" + filepath.Base(filepath.Clean(dir)) + "」"
	fi, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "将新建文件夹" + name + "。", true
	case err == nil && !fi.IsDir():
		return name + "是一个文件，请更换名称或位置。", false
	}
	f, err := os.Open(dir)
	if err != nil {
		return "无法打开文件夹" + name + "，请更换名称或位置。", false
	}
	defer f.Close()
	if names, _ := f.Readdirnames(1); len(names) > 0 {
		return "文件夹" + name + "中已有文件，请更换名称或位置。", false
	}
	return "文件夹" + name + "为空，作品将建在其中。", true
}

// create makes a work where the dialog says.
func (a *App) create() error {
	dir := kit.Trim(a.books.at)
	if say, fits := placeFor(dir); !fits {
		return errors.New(say)
	}
	name := kit.Trim(a.name)
	if name == "" {
		name = filepath.Base(dir)
	}
	s, err := workspace.Create(dir, name, a.genre)
	if err != nil {
		return err
	}
	if talk, pen := a.lastModels(); talk.Provider != "" {
		w := s.Work()
		w.Discussion, w.Writing = talk, pen
		a.warn(s.SaveWork(w))
	}
	a.books.creating = false
	return a.attach(s, dir, true)
}

// lastModels are the models a new work starts with: the open work's, or
// else those of the work last opened, so the author does not choose the
// same model again for each work. A service since removed is not carried.
func (a *App) lastModels() (talk, pen workspace.Selection) {
	known := func(sel workspace.Selection) bool {
		return sel.Model != "" && slices.ContainsFunc(a.ai.Providers(), func(p aiport.Provider) bool { return p.ID == sel.Provider })
	}
	var works []func() (workspace.Work, error)
	if a.Store != nil {
		works = append(works, func() (workspace.Work, error) { return a.Store.Work(), nil })
	}
	for _, b := range a.shelf.Recent() {
		works = append(works, func() (workspace.Work, error) { return workspace.ReadWork(b.Path) })
	}
	for _, read := range works {
		if w, err := read(); err == nil && known(w.Discussion) {
			if !known(w.Writing) {
				w.Writing = workspace.Selection{}
			}
			return w.Discussion, w.Writing
		}
	}
	return
}

// addBook puts a work already on disk on the shelf.
func (a *App) addBook(path string) {
	b, added, err := a.shelf.Add(path)
	if err != nil {
		a.SetError(err)
		return
	}
	if !added {
		a.status.Hint("该作品已在书架上")
		return
	}
	a.saveShelf()
	a.status.Did("已加入书架：" + b.Name)
}

// relocate points a work whose folder moved at where it is now.
func (a *App) relocate(from, to string) {
	if err := a.shelf.RelocateWork(from, to); err != nil {
		a.SetError(err)
		return
	}
	a.saveShelf()
	a.lookForBooks()
}

func (a *App) removeBook(path string) {
	a.shelf.Remove(path)
	a.saveShelf()
	delete(a.books.missing, path)
}

// renameBook changes a work's name, not its folder.
func (a *App) renameBook(path, name string) error {
	name = kit.Trim(name)
	if name == "" {
		return fmt.Errorf("作品名不能为空")
	}
	if a.Store != nil && shelf.SameFolder(a.Store.Root(), path) {
		w := a.Store.Work()
		w.Name = name
		if err := a.Store.SaveWork(w); err != nil {
			return err
		}
		a.remember(false)
		return nil
	}
	s, err := workspace.Open(path)
	if err != nil {
		return err
	}
	defer s.Close()
	w := s.Work()
	w.Name = name
	if err = s.SaveWork(w); err != nil {
		return err
	}
	a.shelf.Put(shelf.DescribeOpen(s))
	a.saveShelf()
	return nil
}

// pickInto asks for a folder off the view thread and hands it to then.
// False means no system picker is configured.
func (a *App) pickInto(c *ui.Context, title string, then func(path string)) bool {
	if a.PickFolder == nil {
		return false
	}
	pick, sv := a.PickFolder, c.Services()
	a.jobs.start(sv, func() func() {
		path, err := pick(title)
		return func() {
			if err != nil {
				a.SetError(err)
			} else if path != "" {
				then(path)
			}
		}
	})
	return true
}

// RevealFolder shows a folder in the system's file manager.
func RevealFolder(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// length says how long a work is.
func length(n int) string {
	if n == 0 {
		return "暂无正文"
	}
	return kit.Words(n)
}

func (a *App) welcome(c *ui.Context) {
	t := a.pal.Theme
	// The shelf has the window to itself: the cards' columns come from its
	// width in this frame, as wide as the column below fits.
	win, _ := c.Size()
	width := min(940, win-2*32)
	ui.Scroll(c).Fill().Children(func() {
		books := a.shelf.Recent()
		if len(books) == 0 {
			a.emptyShelf(c)
			return
		}
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(64, 32, 40, 32).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(940).Gap(24).Label("书架内容").Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.End).Gap(8).Children(func() {
					ui.Column(c).Grow(1).Shrink(1).Gap(6).Children(func() {
						ui.Text(c, "我的作品").FontSize(kit.FsDisplay).FontWeight(700)
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Textf(c, "%d 部 · 作品库：%s", len(books), a.shelf.Library).FontSize(kit.FsControl).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
							if a.PickFolder != nil && a.pal.Ghost(c, "更改", ui.Color{}).Label("更改作品库").Clicked() {
								a.pickInto(c, "选择作品库文件夹", func(p string) {
									a.shelf.Library = p
									a.saveShelf()
								})
							}
						})
					})
					a.viewToggle(c)
					if a.PickFolder != nil && a.pal.Secondary(c, "添加已有作品…").Clicked() {
						a.pickInto(c, "选择已有作品的文件夹", a.addBook)
					}
					if a.pal.Primary(c, "新建作品").Clicked() {
						a.openCreate()
					}
				})
				if a.shelf.Cards {
					cards := ui.Row(c).FillWidth().Wrap().Gap(16)
					cols := max(1, int((width+16)/(220+16)))
					cardWidth := max(220, (width-float32(cols-1)*16)/float32(cols))
					cards.Children(func() {
						for _, b := range books {
							a.bookCard(c, b, cardWidth)
						}
					})
				} else {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						for _, b := range books {
							a.bookRow(c, b)
						}
					})
				}
			})
		})
	})
}

func (a *App) emptyShelf(c *ui.Context) {
	t := a.pal.Theme
	_, h := c.Size()
	about, facts := "与 AI 逐步讨论，定下的内容当场记进设定文档。已确定的设定会被遵守，已放弃的 AI 不再提及。", []string{
		"作品、对话和设定文档均为文件夹中的普通文件，可备份、同步或用其他编辑器打开。",
		"AI 仅在你发送消息时回应，不会自行续写。",
		"API key 保存在系统钥匙串中，不会写入作品文件夹。",
	}
	if !a.aiOn() { // the shelf says nothing of the AI either
		about, facts = "新建一部作品，或添加已有的作品文件夹。", []string{"作品和设定文档均为文件夹中的普通文件，可备份、同步或用其他编辑器打开。"}
	}
	a.pal.Blank(c, kit.IcBook, "从一个想法开始", about, func() {
		ui.Column(c).Padding(10, 0).Gap(6).Children(func() {
			for _, line := range facts {
				ui.Text(c, "· "+line).FontSize(kit.FsNote).TextColor(t.TextMuted)
			}
		})
		ui.Row(c).Gap(8).Padding(10, 0, 0, 0).Justify(ui.End).Children(func() {
			if a.PickFolder != nil && a.pal.Secondary(c, "添加已有作品…").Clicked() {
				a.pickInto(c, "选择已有作品的文件夹", a.addBook)
			}
			if a.pal.Primary(c, "新建作品").Clicked() {
				a.openCreate()
			}
		})
	}).MinHeight(h).Padding(24)
}

// viewToggle switches the shelf between a list and cards.
func (a *App) viewToggle(c *ui.Context) {
	t := a.pal.Theme
	ui.Row(c).Gap(2).Padding(2).Radius(kit.RControl).Background(t.SurfacePressed).Shrink(0).Children(func() {
		for _, v := range []struct {
			icon  *ui.SVG
			label string
			cards bool
		}{{kit.IcList, "列表", false}, {kit.IcGrid, "卡片", true}} {
			on := a.shelf.Cards == v.cards
			b := ui.ButtonBase(c).Size(kit.HSeg, kit.HSeg).Center().Radius(kit.RXs).Label("书架-" + v.label).Tooltip("按" + v.label + "显示")
			fg := t.TextMuted
			if on {
				a.pal.Raise.On(b.Background(a.pal.Canvas))
			}
			if on || b.Hovered() {
				fg = t.Text
			}
			b.Children(func() { kit.Glyph(c, v.icon, kit.IconSm, fg) })
			if b.Clicked() && a.shelf.Cards != v.cards {
				a.shelf.Cards = v.cards
				a.saveShelf()
			}
		}
	})
}

// initial is a work's mark: the first character of its name.
func (a *App) initial(c *ui.Context, name string, gone bool) {
	bg := a.pal.Ink
	if gone {
		bg = a.pal.Dim
	}
	ui.Box(c).Size(kit.HControl, kit.HControl).Radius(kit.RControl).Background(bg).Center().Shrink(0).Children(func() {
		ui.Text(c, string([]rune(name + " ")[:1])).FontSize(kit.FsGroup).FontWeight(600).TextColor(a.pal.OnInk)
	})
}

func (a *App) bookRow(c *ui.Context, b shelf.Book) {
	t := a.pal.Theme
	gone := a.books.missing[b.Path]
	ui.Row(c.Key("b-" + b.Path)).FillWidth().AlignItems(ui.Center).Gap(4).Children(func() {
		open := ui.ButtonBase(c).Row().Grow(1).Shrink(1).AlignItems(ui.Center).Gap(14).Padding(12).Radius(kit.RControl).Label("作品-" + b.Name)
		if open.Hovered() && !gone {
			open.Background(t.SurfaceHover)
		}
		open.Children(func() {
			a.initial(c, b.Name, gone)
			ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
				fg := t.Text
				if gone {
					fg = t.TextMuted
				}
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, b.Name).FontSize(kit.FsGroup).FontWeight(600).TextColor(fg).SingleLine().Ellipsis("…").Shrink(1)
					ui.Text(c, b.Genre+" · "+length(b.Words)).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine()
				})
				ui.Text(c, b.Path).FontSize(kit.FsNote).TextColor(a.pal.Faint).SingleLine().Ellipsis("…")
			})
			if gone {
				ui.Text(c, "找不到文件夹").FontSize(kit.FsNote).TextColor(t.Danger)
			} else {
				ui.Text(c, kit.Opened(b.Opened, time.Now())).FontSize(kit.FsNote).TextColor(a.pal.Faint).SingleLine()
			}
		})
		if open.Clicked() && !gone {
			a.switchWork(b.Path)
		}
		if gone {
			if a.PickFolder != nil && a.pal.Ghost(c, "重新定位…", ui.Color{}).Label("重新定位-"+b.Name).Clicked() {
				from := b.Path
				a.pickInto(c, "选择该作品当前所在的文件夹", func(p string) { a.relocate(from, p) })
			}
			if a.pal.Ghost(c, "从书架移除", ui.Color{}).Label("移除-" + b.Name).Clicked() {
				a.removeBook(b.Path)
			}
			return
		}
		a.bookMenu(c, b)
	})
}

func (a *App) bookCard(c *ui.Context, b shelf.Book, width float32) {
	t := a.pal.Theme
	gone := a.books.missing[b.Path]
	card := ui.ButtonBase(c.Key("c-"+b.Path)).Column().Justify(ui.Start).AlignItems(ui.Stretch).Width(width).Height(168).Padding(16).Gap(8).Radius(kit.RDialog).Border(1, t.Border).Background(t.Surface).Label("作品-" + b.Name)
	if card.Hovered() && !gone {
		card.Background(t.SurfaceHover)
	}
	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
			a.initial(c, b.Name, gone)
			ui.Spacer(c)
			if !gone {
				a.bookMenu(c, b)
			}
		})
		fg := t.Text
		if gone {
			fg = t.TextMuted
		}
		ui.Text(c, b.Name).FontSize(kit.FsGroup).FontWeight(600).TextColor(fg).SingleLine().Ellipsis("…").FillWidth()
		ui.Text(c, b.Genre+" · "+length(b.Words)).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine()
		ui.Spacer(c)
		if gone {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "找不到文件夹").FontSize(kit.FsNote).TextColor(t.Danger).SingleLine().Shrink(0)
				ui.Spacer(c)
				if a.PickFolder != nil && a.pal.Ghost(c, "定位", ui.Color{}).Label("重新定位-"+b.Name).Clicked() {
					from := b.Path
					a.pickInto(c, "选择该作品当前所在的文件夹", func(p string) { a.relocate(from, p) })
				}
				if a.pal.Ghost(c, "移除", ui.Color{}).Label("移除-" + b.Name).Clicked() {
					a.removeBook(b.Path)
				}
			})
		} else {
			ui.Text(c, kit.Opened(b.Opened, time.Now())).FontSize(kit.FsNote).TextColor(a.pal.Faint)
		}
	})
	if card.Clicked() && !gone {
		a.switchWork(b.Path)
	}
}

// bookMenu is a work's "⋯": rename, show its folder, take it off the shelf.
func (a *App) bookMenu(c *ui.Context, b shelf.Book) {
	key := "作品-" + b.Path
	more := kit.IconButton(c, kit.IcMore, "更多-"+b.Name, ui.Color{}).Tooltip("更多操作")
	if a.menu.MenuOpen(key) {
		more.Background(a.pal.Theme.SurfacePressed)
	}
	a.menu.DropMenu(c, a.pal, more, key, func(panel ui.Element) {
		panel.AttachTo(more, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(4, 0, 0, 0).Width(180)
	}, func(m *kit.Menu) {
		if a.Reveal != nil {
			m.Item("打开所在文件夹").Do(func() { a.warn(a.Reveal(b.Path)) })
		}
		m.Item("重命名…").Do(func() { a.books.renaming, a.books.name = b.Path, b.Name })
		m.Item("从书架移除").Do(func() {
			a.removeBook(b.Path)
			a.status.Did("已从书架移除，文件夹未改动：" + b.Path)
		})
	})
}

// createDialog asks for a new work's name, genre and place; the place
// follows the name inside the library until the author picks one.
func (a *App) createDialog(c *ui.Context) {
	t := a.pal.Theme
	v := &a.books
	if !v.creating {
		return
	}
	name := kit.Trim(a.name)
	if name == "" {
		name = "未命名作品"
	}
	if auto := filepath.Join(a.shelf.Library, name); v.at == v.auto {
		v.at, v.auto = auto, auto
	}
	if dir := kit.Trim(v.at); dir != v.looked || v.place == "" {
		v.looked = dir
		v.place, v.fits = placeFor(dir)
	}
	a.pal.Modal(c, &v.creating, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.pal.DialogHead(c, "新建作品", "")
			kit.Labeled(c, "作品名称", func() {
				ui.TextInput(c, &a.name).Label("作品名称").Placeholder("之后可修改").FillWidth().Focus()
			})
			kit.Labeled(c, "体裁", func() {
				ui.Select(c, &a.genre, []string{"网文", "出版小说", "剧本", "非虚构"}).Label("体裁").FillWidth()
			})
			kit.Labeled(c, "位置", func() {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c, &v.at).Label("作品位置").Grow(1)
					if a.PickFolder != nil && a.pal.Secondary(c, "更改…").Clicked() {
						a.pickInto(c, "选择新作品所在的文件夹", func(p string) {
							v.at = filepath.Join(p, filepath.Base(v.at))
						})
					}
				})
				say := ui.Text(c, v.place).FontSize(kit.FsNote).TextColor(t.TextMuted)
				if !v.fits && v.looked != "" {
					say.TextColor(t.Danger)
				}
			})
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if a.pal.Secondary(c, "取消").Clicked() {
					v.creating = false
				}
				if a.pal.Primary(c, "创建").Label("创建作品").Disabled(!v.fits).Clicked() {
					a.SetError(a.create())
				}
			})
		})
	})
}

func (a *App) renameDialog(c *ui.Context) {
	v := &a.books
	open := v.renaming != ""
	if !open {
		return
	}
	a.pal.Modal(c, &open, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.pal.DialogHead(c, "重命名作品", "")
			ui.TextInput(c, &v.name).Label("新作品名").FillWidth().Focus()
			ui.Text(c, "仅修改作品名，文件夹不变。").FontSize(kit.FsNote).TextColor(a.pal.Faint)
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if a.pal.Secondary(c, "取消").Clicked() {
					open = false
				}
				if a.pal.Primary(c, "保存").Label("保存作品名").Disabled(kit.Trim(v.name) == "").Clicked() {
					if err := a.renameBook(v.renaming, v.name); err != nil {
						a.SetError(err)
					} else {
						open = false
					}
				}
			})
		})
	})
	if !open {
		v.renaming = ""
	}
}
