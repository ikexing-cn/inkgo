package desk

import (
	"github.com/egoist/mygo/ui"
	"testing"
	"time"
	"writer/ai"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/lore/loreapi"
	"writer/ext/marks/marksapi"
	"writer/ext/talk/talkapi"
	"writer/workspace"
)

func TestAuthoringDrawAndMenuPointsUseFrameCallChainsAndIsolateFailure(t *testing.T) {
	for _, point := range []string{"beside", "menu"} {
		t.Run(point, func(t *testing.T) {
			calls, before := 0, 0
			target := "fake.body"
			fake := ext.Module{ID: "fake", Setup: func(r *ext.Registrar) {
				if point == "beside" {
					ext.Contribute(r, hostapi.BesideItems, "body", hostapi.DrawItem{Draw: func(c *ui.Context) { calls++; ui.Text(c, "partial"); panic("bad draw") }})
				} else {
					ext.Contribute(r, hostapi.SelectionMenus, "body", hostapi.SelectionMenu{Build: func(_ string) []hostapi.SelectionAction { calls++; panic("bad menu") }})
				}
			}}
			injector := ext.Module{ID: "injector", Requires: []string{"fake"}, Setup: func(r *ext.Registrar) {
				if point == "beside" {
					ext.Before(r, ext.HookOf(ext.CallPoint[hostapi.DrawItem, *ui.Context, struct{}]{Point: hostapi.BesideItems}, target), "trace", func(in **ui.Context) { before++ })
				} else {
					ext.Before(r, ext.HookOf(ext.CallPoint[hostapi.SelectionMenu, string, []hostapi.SelectionAction]{Point: hostapi.SelectionMenus}, target), "trace", func(in *string) { before++ })
				}
			}}
			a := newTestApp(ai.New(""), append(testModules(), fake, injector)...)
			defer a.Close()
			requireExtension(t, a.Open(extensionBook(t)))
			tt := stillTester(func(c *ui.Context) {
				if point == "beside" {
					a.besideItems(c)
				} else {
					var state kit.MenuState
					state.MenuAt(c, a.pal, "fake", func(m *kit.Menu) { a.selectionMenus(m, "name") })
				}
			}, 1266, 780)
			if point == "menu" {
				// Menu callbacks normally execute while drawing a selection menu; use
				// an open host menu to exercise the same frame-thread dispatch.
				tt = stillTester(func(c *ui.Context) {
					state := kit.MenuState{Owner: "fake", X: 10, Y: 10}
					state.MenuAt(c, a.pal, "fake", func(m *kit.Menu) { a.selectionMenus(m, "name") })
				}, 1266, 780)
			}
			tt.Frame()
			tt.Frame()
			if calls != 1 || before != 1 {
				t.Fatal(calls, before)
			}
			if point == "beside" && tt.HasText("partial") {
				t.Fatal("partial draw remained visible")
			}
		})
	}
}

func TestEntryNavigationIsOptional(t *testing.T) {
	a := &App{}
	if a.entryNavigation(func(loreapi.EntryNavigation) { t.Fatal("missing service called") }) {
		t.Fatal("missing service available")
	}
}

func TestSearchBookAndRevisionServicesAreDirectFrameCapabilities(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚")
	search, ok := ext.Lookup(a.extensions, hostapi.SearchService)
	if !ok {
		t.Fatal("search missing")
	}
	content, ok := ext.Lookup(a.extensions, hostapi.EditingService)
	if !ok {
		t.Fatal("book missing")
	}
	revisions, ok := ext.Lookup(a.extensions, hostapi.EditingService)
	if !ok {
		t.Fatal("revisions missing")
	}
	frame := stillTester(func(c *ui.Context) {
		if search.CountText("林晚") != 1 {
			t.Fatal("live text search")
		}
		content.Flashed("none")
		content.ClearFlash("none")
		if err := revisions.Refresh(); err != nil {
			t.Fatal(err)
		}
		search.FindInChapter(a.write.node, "林晚")
		ui.Text(c, "frame capability")
	}, 1266, 780)
	frame.Frame()
	tt.Frame()
}

func TestMarginSourcesAndActionsUseGuardAndIsolateFailures(t *testing.T) {
	for _, stage := range []string{"source", "open", "action"} {
		t.Run(stage, func(t *testing.T) {
			calls := 0
			fake := ext.Module{ID: "fakenotes", Requires: []string{"marks"}, Setup: func(r *ext.Registrar) {
				ext.Contribute(r, marksapi.MarginSources, "notes", marksapi.MarginSource{Notes: func(chapter, text string) []marksapi.MarginItem {
					if stage == "source" {
						calls++
						panic("source failed")
					}
					return []marksapi.MarginItem{{ID: "fakenotes.note", Kind: "fake", Title: "fake note", Start: 0, End: 1, Open: func() { calls++; panic("open failed") }, Actions: []marksapi.NoteAction{{Label: "act", Run: func() { calls++; panic("action failed") }}}}}
				}})
			}}
			a := newTestApp(ai.New(""), append(testModules(), fake)...)
			defer a.Close()
			requireExtension(t, a.Open(extensionBook(t)))
			id, err := a.book.NewChapter("", "chapter", false)
			if err != nil {
				t.Fatal(err)
			}
			a.openNode(id)
			a.page = pageWrite
			tt := stillTester(func(c *ui.Context) {
				notes := a.marginNotes()
				if stage != "source" && calls == 0 {
					for _, n := range notes {
						if n.id == "fakenotes.note" {
							if stage == "open" {
								a.openMarginNote(n)
							} else {
								n.actions[0].run()
							}
						}
					}
				}
				ui.Text(c, "host alive")
			}, 1266, 780)
			tt.Frame()
			tt.Frame()
			ledger := a.extensions.Ledger("fakenotes")
			if calls != 1 || ledger.State != ext.Failed || len(ledger.Errors) != 1 || ledger.Errors[0].Stage != map[bool]string{true: "Original", false: "Guard"}[stage == "source"] {
				t.Fatal(calls, ledger)
			}
			if !tt.HasText("host alive") {
				t.Fatal("host stopped")
			}
		})
	}
}

type failingEntryNavigation struct{ loreapi.EntryNavigation }

func (failingEntryNavigation) Open(workspace.Entry) { panic("entry navigation failed") }

func TestEntryNavigationGuardsTheProvidersOwner(t *testing.T) {
	h, err := ext.New(ext.Options{}, ext.Module{ID: "lore", Setup: func(r *ext.Registrar) {
		ext.Provide(r, loreapi.EntryNavigationService, func(*ext.Session) loreapi.EntryNavigator {
			return loreapi.EntryNavigator{Owner: "lore", Navigation: failingEntryNavigation{}}
		})
	}}, ext.Module{ID: "healthy"})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	requireExtension(t, h.Open(ext.Book{}))
	a := &App{extensions: h}
	tt := stillTester(func(c *ui.Context) {
		a.entryNavigation(func(n loreapi.EntryNavigation) { n.Open(workspace.Entry{}) })
		ui.Text(c, "host alive")
	}, 1000, 700)
	tt.Frame()
	tt.Frame()
	if h.Ledger("lore").State != ext.Failed || h.Ledger("healthy").State != ext.On || !tt.HasText("host alive") {
		t.Fatal(h.Ledger("lore"), h.Ledger("healthy"))
	}
}

func TestPaneBadgeQueriesUseGuard(t *testing.T) {
	fake := ext.Module{ID: "fakebadge", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.PaneTabs, "tab", hostapi.PaneTab{Title: func() string { return "fake badge" }, Count: func() int { panic("badge failed") }})
	}}
	a := newTestApp(ai.New(""), append(testModules(), fake)...)
	defer a.Close()
	requireExtension(t, a.Open(extensionBook(t)))
	a.page = pageWrite
	tt := stillTester(a.View, 1266, 780)
	tt.Frame()
	tt.Frame()
	l := a.extensions.Ledger("fakebadge")
	if l.State != ext.Failed || len(l.Errors) != 1 || l.Errors[0].Stage != "Guard" {
		t.Fatal(l)
	}
}

func TestSelectionActionClickIsGuardedAfterMenuConstruction(t *testing.T) {
	calls := 0
	fake := ext.Module{ID: "fakeaction", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.SelectionMenus, "menu", hostapi.SelectionMenu{Build: func(string) []hostapi.SelectionAction {
			return []hostapi.SelectionAction{{Label: "fake action", Run: func() { calls++; panic("action failed") }}}
		}})
	}}
	a := newTestApp(ai.New(""), append(testModules(), fake)...)
	defer a.Close()
	requireExtension(t, a.Open(extensionBook(t)))
	state := kit.MenuState{Owner: "fake", X: 10, Y: 10}
	tt := stillTester(func(c *ui.Context) { state.MenuAt(c, a.pal, "fake", func(m *kit.Menu) { a.selectionMenus(m, "name") }) }, 1266, 780)
	tt.Frame()
	click(t, tt, "fake action")
	l := a.extensions.Ledger("fakeaction")
	if calls != 1 || l.State != ext.Failed || len(l.Errors) != 1 || l.Errors[0].Stage != "Guard" {
		t.Fatal(calls, l)
	}
}

func TestEntryActionsAreAbsentWithoutNavigationService(t *testing.T) {
	a := &App{}
	actions := checkActionsWithoutNavigation()
	if len(actions) != 2 {
		t.Fatal("entry edit action remained", actions)
	}
	a.showEntry(workspace.Entry{ID: "missing"})
	a.seeName("missing")
	if a.doc.hidden || a.doc.tab != "" || a.write.focus {
		t.Fatal("missing service changed presentation")
	}
}

func TestTalkAndAIServiceMethodsAreDirectFrameCalls(t *testing.T) {
	a, _, _ := writingFixture(t, "正文")
	talk, ok := ext.Lookup(a.extensions, talkapi.Service)
	if !ok {
		t.Fatal("talk missing")
	}
	state, ok := ext.Lookup(a.extensions, hostapi.ExtensionsService)
	if !ok {
		t.Fatal("AI state missing")
	}
	id := talkOf(a).Session
	talkOf(a).Titles[id] = workspace.ConversationTitle{Kind: workspace.TitleWords, Text: "服务测试"}
	frame := stillTester(func(c *ui.Context) {
		if !state.Enabled("ai") {
			t.Fatal("AI should be on")
		}
		a.turnAI(!(true))
		if state.Enabled("ai") {
			t.Fatal("AI state was cached")
		}
		a.turnAI(!(false))
		title, exists := talk.TalkTitle(id)
		if !exists || title != "服务测试" {
			t.Fatal(title, exists)
		}
		talk.OpenTalk(id)
		if a.page != pageTalk || talkOf(a).Session != id {
			t.Fatal("conversation not opened")
		}
		ui.Text(c, "direct services")
	}, 1266, 780)
	frame.Frame()
}

func TestExistingServicesOwnMenusPaneCategoriesAndContentFlashes(t *testing.T) {
	a, _, _ := writingFixture(t, "正文")
	desktop, _ := ext.Lookup(a.extensions, hostapi.NavigationService)
	native, _ := ext.Lookup(a.extensions, hostapi.NativeUIService)
	pane, _ := ext.Lookup(a.extensions, hostapi.PaneService)
	content, _ := ext.Lookup(a.extensions, hostapi.EditingService)
	now := time.Now()
	frame := stillTester(func(c *ui.Context) {
		a.menu = kit.MenuState{Owner: "service-menu"}
		if !native.MenuOpen("service-menu") || native.MenuOpen("other") {
			t.Fatal("shared menu state")
		}
		a.flash["entry"] = now
		if at, ok := content.Flashed("entry"); !ok || at != now {
			t.Fatal(at, ok)
		}
		content.ClearFlash("entry")
		if _, ok := content.Flashed("entry"); ok {
			t.Fatal("flash remained")
		}
		desktop.ShowPage("cats.categories")
		if a.page != pageCategories {
			t.Fatal("category page missing")
		}
		a.page = pagePlan
		a.doc.hidden, talkOf(a).Talks, a.write.focus = true, true, true
		if a.paneOpen() {
			a.pages[pagePlan].togglePane()
		}
		pane.RevealPane("setting")
		if !a.paneOpen() || a.doc.tab != paneSetting || talkOf(a).Talks || !a.write.focus {
			t.Fatal("reveal semantics")
		}
		a.page = pageWrite
		a.doc.hidden, talkOf(a).Talks, a.write.focus = true, true, true
		pane.Show("setting")
		if a.doc.hidden || talkOf(a).Talks || a.write.focus || a.doc.tab != paneSetting {
			t.Fatal("show semantics")
		}
		talkOf(a).Talks = true
		pane.Show("talk")
		if !talkOf(a).Talks {
			t.Fatal("talk list closed when selecting talk")
		}
		ui.Text(c, "existing service capabilities")
	}, 1266, 780)
	frame.Frame()
}

func TestSplitServicesUsedInsideCallKeepContributionFailureIsolated(t *testing.T) {
	for _, service := range []string{"talk", "search", "ai"} {
		t.Run(service, func(t *testing.T) {
			calls := 0
			fake := ext.Module{ID: "serviceuser", Optional: []string{"talk"}, Setup: func(r *ext.Registrar) {
				talk := ext.UseOptional(r, talkapi.Service)
				search := ext.Use(r, hostapi.SearchService)
				state := ext.Use(r, hostapi.ExtensionsService)
				ext.Contribute(r, hostapi.BesideItems, "body", hostapi.DrawItem{Draw: func(c *ui.Context) {
					switch service {
					case "talk":
						if svc, ok := talk.Get(); ok {
							svc.TalkTitle("missing")
						}
					case "search":
						search.Get().CountText("正文")
					case "ai":
						state.Get().Enabled("ai")
					}
					calls++
					panic("consumer failed")
				}})
			}}
			a := newTestApp(ai.New(""), append(testModules(), fake)...)
			defer a.Close()
			requireExtension(t, a.Open(extensionBook(t)))
			frame := stillTester(func(c *ui.Context) { a.besideItems(c); ui.Text(c, "host alive") }, 1266, 780)
			frame.Frame()
			frame.Frame()
			if calls != 1 || a.extensions.Ledger("serviceuser").State != ext.Failed || !frame.HasText("host alive") {
				t.Fatal(calls, a.extensions.Ledger("serviceuser"))
			}
		})
	}
}
