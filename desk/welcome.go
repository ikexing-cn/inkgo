package desk

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"
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
	// rather than following the name.
	creating bool
	at, auto string
	// The work being renamed and its new name.
	renaming, name string
}

// loadShelf reads the shelf beside the model configuration, starting one
// from the library the first time.
func (a *App) loadShelf() {
	a.books.file = shelf.File(a.ConfigFile)
	s, fresh, err := shelf.Load(a.books.file)
	if err != nil {
		a.books.broken = true
		a.Error = "作品清单读不了，这次不会改动它：" + err.Error()
		return
	}
	if fresh {
		s = shelf.Start(Library())
		for i, b := range s.Books {
			if d, err := describe(b.Path); err == nil {
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
		if !shelf.IsWork(b.Path) {
			a.books.missing[b.Path] = true
		}
	}
}

// describe reads what the shelf shows of the work at path.
func describe(path string) (shelf.Book, error) {
	s, err := workspace.Open(path)
	if err != nil {
		return shelf.Book{}, err
	}
	defer s.Close()
	return book(s), nil
}

func book(s *workspace.Store) shelf.Book {
	w := s.Work()
	b := shelf.Book{Path: s.Root(), Name: w.Name, Genre: w.Genre}
	counts, _ := s.Counts()
	for _, n := range counts {
		b.Words += n
	}
	return b
}

// remember puts the open work on the shelf as it is now.
func (a *App) remember(opened bool) {
	if a.Store == nil {
		return
	}
	b := book(a.Store)
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
	if len(recent) == 0 || recent[0].Opened.IsZero() || !shelf.IsWork(recent[0].Path) {
		return
	}
	a.setError(a.Open(recent[0].Path))
}

// switchWork opens another work from the shelf.
func (a *App) switchWork(path string) {
	if !shelf.IsWork(path) {
		a.books.missing[path] = true
		a.Error = "找不到这部作品的文件夹：" + path
		return
	}
	a.setError(a.Open(path))
}

// openCreate starts the new-work dialog.
func (a *App) openCreate() {
	a.name, a.genre = "", "网文"
	a.books.creating, a.books.at, a.books.auto = true, "", ""
}

// create makes a work where the dialog says.
func (a *App) create() error {
	dir := trim(a.path)
	if a.books.creating {
		dir = trim(a.books.at)
	}
	if dir == "" {
		return fmt.Errorf("请选择作品放在哪里")
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s 里已经有文件了，换个名字或位置", dir)
	}
	name := trim(a.name)
	if name == "" {
		name = filepath.Base(dir)
	}
	s, err := workspace.Create(dir, name, a.genre)
	if err != nil {
		return err
	}
	a.books.creating = false
	return a.attach(s, dir, true)
}

// addBook puts a work already on disk on the shelf.
func (a *App) addBook(path string) {
	if !shelf.IsWork(path) {
		a.Error = "这个文件夹里没有作品（缺少 work.yaml）：" + path
		return
	}
	if a.shelf.Find(path) >= 0 {
		a.notice = "这部作品已经在书架上了"
		return
	}
	b, err := describe(path)
	if err != nil {
		a.setError(err)
		return
	}
	a.shelf.Put(b)
	a.saveShelf()
	a.notice = "已加入书架：" + b.Name
}

// relocate points a work whose folder moved at where it is now.
func (a *App) relocate(from, to string) {
	if !shelf.IsWork(to) {
		a.Error = "这个文件夹里没有作品（缺少 work.yaml）：" + to
		return
	}
	b, err := describe(to)
	if err != nil {
		a.setError(err)
		return
	}
	a.shelf.Relocate(from, b.Path)
	a.shelf.Put(b)
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
	name = trim(name)
	if name == "" {
		return fmt.Errorf("作品名不能为空")
	}
	if a.Store != nil && sameFolder(a.Store.Root(), path) {
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
	a.shelf.Put(book(s))
	a.saveShelf()
	return nil
}

func sameFolder(x, y string) bool {
	var s shelf.Shelf
	s.Put(shelf.Book{Path: x})
	return s.Find(y) == 0
}

// pickInto asks for a folder off the view thread and hands it to then.
func (a *App) pickInto(c *ui.Context, title string, then func(path string)) {
	pick := a.PickFolder
	go func() {
		path, err := pick(title)
		a.picks <- func() {
			if err != nil {
				a.Error = err.Error()
			} else if path != "" {
				then(path)
			}
		}
		c.Invalidate()
	}()
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

func revealLabel() string {
	switch runtime.GOOS {
	case "windows":
		return "在资源管理器中显示"
	case "darwin":
		return "在访达中显示"
	}
	return "打开所在文件夹"
}

// opened says when a work was last opened, briefly.
func opened(t time.Time) string {
	if t.IsZero() {
		return "还没打开过"
	}
	now := time.Now()
	t = t.Local()
	y1, m1, d1 := now.Date()
	y2, m2, d2 := t.Date()
	switch {
	case y1 == y2 && m1 == m2 && d1 == d2:
		return "今天 " + t.Format("15:04")
	case now.Sub(t) < 48*time.Hour && now.AddDate(0, 0, -1).Day() == d2:
		return "昨天 " + t.Format("15:04")
	case y1 == y2:
		return t.Format("01-02")
	}
	return t.Format("2006-01-02")
}

// length says how long a work is.
func length(n int) string {
	if n == 0 {
		return "还没有正文"
	}
	return words(n)
}

func (a *App) welcome(c *ui.Context) {
	t := a.pal.theme
	ui.Scroll(c).Fill().Children(func() {
		books := a.shelf.Recent()
		if len(books) == 0 {
			a.emptyShelf(c)
			return
		}
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(48, 32).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(960).Gap(22).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.End).Gap(10).Children(func() {
					ui.Column(c).Grow(1).Shrink(1).Gap(6).Children(func() {
						ui.Text(c, "我的作品").FontSize(26).Bold()
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Textf(c, "%d 部 · 作品库：%s", len(books), a.shelf.Library).FontSize(13).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
							if a.PickFolder != nil && ghost(c, "更改", a.pal.ink).Label("更改作品库").Clicked() {
								a.pickInto(c, "选择作品库文件夹", func(p string) {
									a.shelf.Library = p
									a.saveShelf()
								})
							}
						})
					})
					a.viewToggle(c)
					if a.PickFolder != nil && ui.Button(c, "添加已有作品…").Clicked() {
						a.pickInto(c, "选择已有作品的文件夹", a.addBook)
					}
					if ui.PrimaryButton(c, "新建作品").Label("新建作品").Clicked() {
						a.openCreate()
					}
				})
				if a.shelf.Cards {
					ui.Row(c).FillWidth().Wrap().Gap(16).Children(func() {
						for _, b := range books {
							a.bookCard(c, b)
						}
					})
				} else {
					a.pal.cardBox(c).FillWidth().Padding(6).Gap(0).Children(func() {
						for i, b := range books {
							if i > 0 {
								ui.Box(c).FillWidth().Height(1).Background(a.pal.line)
							}
							a.bookRow(c, b)
						}
					})
				}
			})
		})
	})
}

func (a *App) emptyShelf(c *ui.Context) {
	t := a.pal.theme
	ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(72, 24).Gap(28).Children(func() {
		ui.Column(c).Width(560).Gap(10).Children(func() {
			ui.Text(c, "从一个念头开始").FontSize(30).Bold()
			ui.Text(c, "和 AI 慢慢聊，讨论会在后台整理成设定文档。你定下的会被遵守，放下的 AI 不再提起。").FontSize(15).TextColor(t.TextMuted).LineHeight(1.6)
		})
		ui.Row(c).Width(560).Gap(10).Children(func() {
			if ui.PrimaryButton(c, "新建作品").Label("新建作品").Clicked() {
				a.openCreate()
			}
			if a.PickFolder != nil && ui.Button(c, "添加已有作品…").Clicked() {
				a.pickInto(c, "选择已有作品的文件夹", a.addBook)
			}
		})
		ui.Column(c).Width(560).Gap(6).Children(func() {
			for _, line := range []string{
				"作品、对话和设定文档都是你文件夹里的普通文件，可以备份、同步或用别的编辑器打开。",
				"AI 只在你发送时回应，不会自己往下写。",
				"API key 存在系统钥匙串里，不会写进作品文件夹。",
			} {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, "·").TextColor(t.Accent).Bold()
					ui.Text(c, line).FontSize(13).TextColor(t.TextMuted).Grow(1)
				})
			}
		})
	})
}

// viewToggle switches the shelf between a list and cards.
func (a *App) viewToggle(c *ui.Context) {
	t := a.pal.theme
	ui.Row(c).Gap(2).Padding(2).Radius(8).Border(1, t.Border).Children(func() {
		for _, v := range []struct {
			icon  *ui.SVG
			label string
			cards bool
		}{{icList, "列表", false}, {icGrid, "卡片", true}} {
			b := iconButton(c, v.icon, "书架-"+v.label, ui.Color{}).Tooltip("按" + v.label + "显示")
			if a.shelf.Cards == v.cards {
				b.Background(t.SurfacePressed)
			}
			if b.Clicked() && a.shelf.Cards != v.cards {
				a.shelf.Cards = v.cards
				a.saveShelf()
			}
		}
	})
}

// initial is a work's mark: the first character of its name.
func (a *App) initial(c *ui.Context, name string, size float32, gone bool) {
	bg := a.pal.ink
	if gone {
		bg = a.pal.faint
	}
	ui.Box(c).Size(size, size).Radius(size / 4).Background(bg).Center().Shrink(0).Children(func() {
		ui.Text(c, string([]rune(name + " ")[:1])).FontSize(size * 0.45).Bold().TextColor(a.pal.onInk)
	})
}

func (a *App) bookRow(c *ui.Context, b shelf.Book) {
	t := a.pal.theme
	gone := a.books.missing[b.Path]
	ui.Row(c).Key("b-" + b.Path).FillWidth().AlignItems(ui.Center).Gap(4).Children(func() {
		open := ui.ButtonBase(c).Row().Grow(1).Shrink(1).AlignItems(ui.Center).Gap(14).Padding(12, 12).Radius(8).Label("作品-" + b.Name).Disabled(gone)
		if open.Hovered() && !gone {
			open.Background(t.SurfaceHover)
		}
		open.Children(func() {
			a.initial(c, b.Name, 36, gone)
			ui.Column(c).Grow(1).Shrink(1).Gap(3).Children(func() {
				fg := t.Text
				if gone {
					fg = a.pal.faint
				}
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, b.Name).FontSize(15).FontWeight(600).TextColor(fg).SingleLine().Ellipsis("…").Shrink(1)
					ui.Text(c, b.Genre+" · "+length(b.Words)).FontSize(12.5).TextColor(t.TextMuted).SingleLine()
				})
				ui.Text(c, b.Path).FontSize(12).TextColor(a.pal.faint).SingleLine().Ellipsis("…")
			})
			if gone {
				ui.Text(c, "找不到文件夹").FontSize(12.5).TextColor(t.Danger)
			} else {
				ui.Text(c, opened(b.Opened)).FontSize(12.5).TextColor(a.pal.faint).SingleLine()
			}
		})
		if open.Clicked() {
			a.switchWork(b.Path)
		}
		if gone {
			if a.PickFolder != nil && ghost(c, "重新定位…", a.pal.ink).Label("重新定位-"+b.Name).Clicked() {
				from := b.Path
				a.pickInto(c, "这部作品现在在哪里", func(p string) { a.relocate(from, p) })
			}
			if ghost(c, "从书架移除", t.TextMuted).Label("移除-" + b.Name).Clicked() {
				a.removeBook(b.Path)
			}
			return
		}
		a.bookMenu(c, b)
	})
}

func (a *App) bookCard(c *ui.Context, b shelf.Book) {
	t := a.pal.theme
	gone := a.books.missing[b.Path]
	card := ui.ButtonBase(c).Key("c-"+b.Path).Column().Justify(ui.Start).AlignItems(ui.Stretch).Width(220).Height(168).Padding(16).Gap(8).Radius(12).Border(1, t.Border).Background(t.Surface).Label("作品-" + b.Name).Disabled(gone)
	if card.Hovered() && !gone {
		card.Background(t.SurfaceHover)
	}
	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
			a.initial(c, b.Name, 40, gone)
			ui.Spacer(c)
			if !gone {
				a.bookMenu(c, b)
			}
		})
		fg := t.Text
		if gone {
			fg = a.pal.faint
		}
		ui.Text(c, b.Name).FontSize(15.5).FontWeight(600).TextColor(fg).SingleLine().Ellipsis("…").FillWidth()
		ui.Text(c, b.Genre+" · "+length(b.Words)).FontSize(12.5).TextColor(t.TextMuted).SingleLine()
		ui.Spacer(c)
		if gone {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "找不到文件夹").FontSize(12).TextColor(t.Danger)
				ui.Spacer(c)
				if a.PickFolder != nil && ghost(c, "定位", a.pal.ink).Label("重新定位-"+b.Name).Clicked() {
					from := b.Path
					a.pickInto(c, "这部作品现在在哪里", func(p string) { a.relocate(from, p) })
				}
				if ghost(c, "移除", t.TextMuted).Label("移除-" + b.Name).Clicked() {
					a.removeBook(b.Path)
				}
			})
		} else {
			ui.Text(c, opened(b.Opened)).FontSize(12).TextColor(a.pal.faint)
		}
	})
	if card.Clicked() {
		a.switchWork(b.Path)
	}
}

// bookMenu is a work's "⋯": rename, show its folder, take it off the shelf.
func (a *App) bookMenu(c *ui.Context, b shelf.Book) {
	iconButton(c, icMore, "更多-"+b.Name, ui.Color{}).Tooltip("更多操作").Menu(func(m *ui.Menu) {
		if m.Item("重命名…").Chosen() {
			a.books.renaming, a.books.name = b.Path, b.Name
		}
		if a.Reveal != nil && m.Item(revealLabel()).Chosen() {
			a.warn(a.Reveal(b.Path))
		}
		m.Separator()
		if m.Item("从书架移除").Chosen() {
			a.removeBook(b.Path)
			a.notice = "已从书架移除，文件夹没有动：" + b.Path
		}
	})
}

// createDialog asks for a new work's name, genre and place; the place
// follows the name inside the library until the author picks one.
func (a *App) createDialog(c *ui.Context) {
	t := a.pal.theme
	v := &a.books
	if !v.creating {
		return
	}
	name := trim(a.name)
	if name == "" {
		name = "未命名作品"
	}
	if auto := filepath.Join(a.shelf.Library, name); v.at == v.auto {
		v.at, v.auto = auto, auto
	}
	ui.Modal(c, &v.creating, func() {
		ui.Column(c).Width(500).Gap(16).Children(func() {
			ui.Text(c, "新建作品").FontSize(17).FontWeight(600)
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Gap(6).Children(func() {
					ui.Text(c, "作品名称").FontSize(13).TextColor(t.TextMuted)
					ui.TextInput(c, &a.name).Label("作品名称").Placeholder("可以之后再改").FillWidth().Focus()
				})
				ui.Column(c).Width(140).Gap(6).Children(func() {
					ui.Text(c, "体裁").FontSize(13).TextColor(t.TextMuted)
					ui.Select(c, &a.genre, []string{"网文", "出版小说", "剧本", "非虚构"}).Label("体裁").FillWidth()
				})
			})
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, "放在").FontSize(13).TextColor(t.TextMuted)
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					ui.TextInput(c, &v.at).Label("作品位置").Grow(1)
					if a.PickFolder != nil && ui.Button(c, "更改…").Clicked() {
						a.pickInto(c, "新作品放在哪个文件夹里", func(p string) {
							v.at = filepath.Join(p, filepath.Base(v.at))
						})
					}
				})
				ui.Text(c, "会新建这个文件夹；它必须不存在或是空的。").FontSize(12).TextColor(a.pal.faint)
			})
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if ui.Button(c, "取消").Clicked() {
					v.creating = false
				}
				if ui.PrimaryButton(c, "创建").Label("创建作品").Disabled(trim(v.at) == "").Clicked() {
					a.setError(a.create())
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
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(420).Gap(14).Children(func() {
			ui.Text(c, "重命名作品").FontSize(17).FontWeight(600)
			ui.TextInput(c, &v.name).Label("新作品名").FillWidth().Focus()
			ui.Text(c, "只改作品名，文件夹不动。").FontSize(12).TextColor(a.pal.faint)
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if ui.Button(c, "取消").Clicked() {
					open = false
				}
				if ui.PrimaryButton(c, "保存").Label("保存作品名").Disabled(trim(v.name) == "").Clicked() {
					if err := a.renameBook(v.renaming, v.name); err != nil {
						a.setError(err)
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
