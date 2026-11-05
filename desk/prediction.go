package desk

import (
	"context"
	"sync/atomic"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
)

type predictionFeed struct {
	value    atomic.Pointer[hostapi.Prediction]
	revision uint64
}
type chapterPrediction struct {
	feed    *predictionFeed
	chapter string
}

func (p chapterPrediction) Predict(ctx context.Context, before string) (string, error) {
	if v := p.feed.value.Load(); v != nil {
		return v.Predictor(p.chapter).Predict(ctx, before)
	}
	return "", nil
}
func (a *App) feedPrediction() {
	revision := a.extensions.Changes()
	if revision != a.prediction.revision {
		a.prediction.revision = revision
		items := ext.Items(a.extensions, hostapi.Predictions)
		var value *hostapi.Prediction
		if len(items) > 0 {
			value = &items[0].Value
		}
		a.prediction.value.Store(value)
	}
	value := a.prediction.value.Load()
	for _, chapter := range a.write.open {
		ed := chapter.ed
		if ed == nil {
			continue
		}
		if ed.NoAI() != (value == nil) {
			ed.SetNoAI(value == nil)
		}
		if value != nil {
			ed.SetPredictOnDemand(value.OnDemand())
		}
	}
}

func (a *App) predictionNote() string {
	if v := a.prediction.value.Load(); v != nil {
		return v.Note()
	}
	return ""
}

var _ editor.Predictor = chapterPrediction{}
