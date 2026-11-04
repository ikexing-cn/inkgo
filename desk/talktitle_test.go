package desk

// talkNameOf is the open talk's title as the list shows it.
func talkNameOf(a *App) string {
	name, _ := talkOf(a).TalkName(talkOf(a).Session)
	return name
}
