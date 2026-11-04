package talk

// Refresh runs on the frame thread.
func (a *Panel) Refresh() error {
	if a.Store == nil {
		return nil
	}
	var err error
	if a.Events, err = a.Store.Events(a.Session); err != nil {
		return err
	}
	if a.Sessions, err = a.Store.Sessions(); err != nil {
		return err
	}
	a.Titles[a.Session], a.Blank[a.Session] = a.TalkTitle(a.Events), len(a.Events) == 0
	if a.Turn.Session != a.Session {
		delete(a.Titles, a.Turn.Session) // the last turn may have gone on there since: read it again
	}
	for _, s := range a.Sessions {
		if _, ok := a.Titles[s]; !ok {
			events, err := a.Store.Events(s)
			if err != nil {
				return err
			}
			a.Titles[s], a.Blank[s] = a.TalkTitle(events), len(events) == 0
		}
	}
	return nil
}
