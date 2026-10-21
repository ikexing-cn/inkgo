package editor

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"
)

// Predictor predicts how the text before the caret goes on. What it
// returns is only shown; the text changes when a human presses Tab.
type Predictor interface {
	Predict(ctx context.Context, before string) (string, error)
}

type predictor struct {
	ghost                 string
	ghostAt, ghostVer     int
	predictVer, predictAt int
	predicting, asked     bool
	stop                  context.CancelFunc
	results               chan prediction
}

type prediction struct {
	text    string
	err     error
	ver, at int
}

func (e *Editor) canPredict() bool {
	return !e.opts.ReadOnly && !e.opts.NoAI && e.opts.Predictor != nil
}

func (p *predictor) ask(e *Editor, c *ui.Context) {
	if !e.canPredict() || !e.focused || p.predicting || e.caret != e.anchor || e.preedit != "" ||
		p.predictVer == e.Doc.textVer && p.predictAt == e.caret {
		return
	}
	if e.opts.PredictOnDemand && !p.asked || e.compOpen && !p.asked {
		return // while completions are offered, Tab is theirs
	}
	if !p.asked {
		if e.caret < e.Doc.Len() && e.Doc.text[e.caret] != '\n' {
			return // predictions continue a paragraph, not the middle of one
		}
		if wait := e.opts.PredictDelay - time.Since(e.lastEdit); wait > 0 {
			c.After(wait)
			return
		}
	}
	p.asked = false
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	p.stop, p.predicting = cancel, true
	p.predictVer, p.predictAt = e.Doc.textVer, e.caret
	ver, at := e.Doc.textVer, e.caret
	from := max(0, at-2000)
	before := e.Doc.Slice(from, at)
	model, results, sv := e.opts.Predictor, p.results, c.Services()
	go func() {
		s, err := model.Predict(ctx, before)
		results <- prediction{text: singleLine(s), err: err, ver: ver, at: at}
		sv.Invalidate()
	}()
}

func (p *predictor) poll(e *Editor) {
	for {
		select {
		case r := <-p.results:
			if r.ver != p.predictVer || r.at != p.predictAt {
				continue // a request cancelled since
			}
			p.predicting, p.stop = false, nil
			if r.err == nil && r.ver == e.Doc.textVer && r.at == e.caret && r.text != "" {
				p.ghost, p.ghostAt, p.ghostVer = r.text, r.at, r.ver
			}
		default:
			return
		}
	}
}

func (p *predictor) cancel() {
	if p.stop != nil {
		p.stop()
		p.stop = nil
	}
	p.predicting = false
}

func (p *predictor) valid(e *Editor) bool {
	return !e.opts.NoAI && p.ghost != "" && p.ghostAt == e.caret && e.anchor == e.caret &&
		p.ghostVer == e.Doc.textVer && e.preedit == ""
}

func (p *predictor) dismiss(e *Editor) {
	p.ghost = ""
	p.cancel()
	p.predictVer, p.predictAt = e.Doc.textVer, e.caret
}

// DismissGhost drops the prediction shown, without asking again here.
func (e *Editor) DismissGhost() { e.predict.dismiss(e) }

// RequestPrediction asks the predictor for a prediction at the caret now,
// wherever the caret is, as a shortcut does when predictions come on
// demand.
func (e *Editor) RequestPrediction() {
	if !e.canPredict() {
		return
	}
	e.predict.cancel()
	e.predict.ghost, e.predict.asked, e.predict.predictVer, e.compOpen = "", true, -1, false
}
