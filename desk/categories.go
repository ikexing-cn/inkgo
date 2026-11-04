package desk

// firstCategory is where an entry the author writes or takes from the
// prose starts: the work's first category, 人物 unless they changed it.
// What is taken from the talk starts in 其他 instead.
func (a *App) firstCategory() string {
	if cats := a.Store.Work().Categories; len(cats) > 0 {
		return cats[0].Name
	}
	return a.Store.Work().Language.OtherCategory()
}

func (a *App) categoryNames() []string {
	var out []string
	for _, cat := range a.Store.Work().Categories {
		out = append(out, cat.Name)
	}
	return out
}
