package desk

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// What a job hands back runs on the next poll, in the order it was handed
// back, and nothing runs before it.
func TestJobsRunInOrderOnPoll(t *testing.T) {
	var j jobs
	var ran []int
	j.post(ui.Services{}, func() { ran = append(ran, 1) })
	j.post(ui.Services{}, func() { ran = append(ran, 2) })
	if len(ran) != 0 || !j.pending() {
		t.Fatalf("ran %v before a poll", ran)
	}
	j.poll()
	if !slices.Equal(ran, []int{1, 2}) {
		t.Fatalf("ran %v", ran)
	}
	if j.pending() {
		t.Fatal("a poll left a function waiting")
	}
	j.poll()
	if !slices.Equal(ran, []int{1, 2}) {
		t.Fatalf("a second poll ran %v", ran)
	}
}

// Many workers handing back at once do not wait for the view thread.
func TestJobsHandBackFromManyGoroutines(t *testing.T) {
	var j jobs
	const n = 100
	var ran atomic.Int64
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j.post(ui.Services{}, func() { ran.Add(1) })
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handing back blocked")
	}
	j.poll()
	if got := ran.Load(); got != n {
		t.Fatalf("ran %d of %d", got, n)
	}
	if j.pending() {
		t.Fatal("a function stayed after the poll")
	}
}

// A job asks the window for the frame that runs what it hands back.
func TestAJobAsksForAFrame(t *testing.T) {
	var j jobs
	var sv ui.Services
	tt := ui.NewTester(func(c *ui.Context) { sv = c.Services(); j.poll() }, 64, 64)
	tt.Frame()
	j.start(sv, func() func() { return func() {} })
	for end := time.Now().Add(3 * time.Second); !tt.FramePending(); {
		if time.Now().After(end) {
			t.Fatal("the job did not ask for a frame")
		}
		time.Sleep(time.Millisecond)
	}
	tt.Frame()
	if j.pending() {
		t.Fatal("the frame did not run the job")
	}
}
