package workspace

import "strings"

// Look is one thing a reply looked up or noted on its way: what it did,
// and what came back, empty while it runs.
type Look struct {
	Label  string
	Result Event
}

// Looks are what the reply to the author's words userID looked up among
// events, in the order it did; for "" all of them, as the turn in flight
// has them. What it proposed in the prose is not among them: that shows
// as a card of its own.
func Looks(events []Event, userID string) []Look {
	results := map[string]Event{}
	for _, e := range events {
		if e.Role == RoleResult && (userID == "" || e.ReplyTo == userID) {
			results[e.Call] = e
		}
	}
	var out []Look
	for _, e := range events {
		if e.Role == RoleCall && (userID == "" || e.ReplyTo == userID) {
			for _, call := range e.Calls {
				if call.Name != ProposeTool {
					out = append(out, Look{call.Label, results[call.ID]})
				}
			}
		}
	}
	return out
}

// Spent is what the requests of the turn reply ends used together: its
// own and each before it that looked something up. A reply from before
// each request was kept apart holds them all, and those before it none.
// The bool reports whether the reply has recorded usage.
func Spent(events []Event, reply Event) (Usage, bool) {
	if reply.Usage == nil {
		return Usage{}, false
	}
	sum := *reply.Usage
	for _, e := range events {
		if e.Role == RoleCall && e.ReplyTo == reply.ReplyTo && e.Usage != nil {
			sum = sum.Add(*e.Usage)
		}
	}
	return sum, true
}

// Requests is how many requests the turn reply ends took, as far as their
// usage tells: each step that looked something up, and the reply's own. A
// reply from before each request was kept apart counts one, as its usage
// is of them all.
func Requests(events []Event, reply Event) int {
	n := 1
	for _, e := range events {
		if e.Role == RoleCall && e.ReplyTo == reply.ReplyTo && e.Usage != nil {
			n++
		}
	}
	return n
}

// FateOf is what came of the proposal of call, as events from from on
// tell, or "" while it is undecided.
func FateOf(events []Event, from int, call string) string {
	for _, e := range events[from:] {
		if e.Role == RoleFate && e.Call == call {
			return e.Fate
		}
	}
	return ""
}

// DigestSections splits a digest at its headings, the lines that open with "## ",
// into each heading and what is under it; what comes before the first has
// none.
func DigestSections(text string) [][2]string {
	var out [][2]string
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if head, ok := strings.CutPrefix(strings.TrimSpace(line), "## "); ok {
			out = append(out, [2]string{strings.TrimSpace(head), ""})
			continue
		}
		if len(out) == 0 {
			out = append(out, [2]string{})
		}
		s := &out[len(out)-1]
		s[1] = strings.TrimPrefix(s[1]+"\n"+line, "\n")
	}
	for n := range out {
		out[n][1] = strings.TrimSpace(out[n][1])
	}
	return out
}
