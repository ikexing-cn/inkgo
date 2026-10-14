// Package aiport is what the interface asks of the AI: the services of
// models this machine has, and the talk, the bookkeeper and the pen of a
// work. Package desk uses the AI through it alone, so that desk imports
// nothing that sends; package ai fills it.
package aiport

import (
	"context"

	"writer/workspace"
)

// Service is the AI of this machine. Catalog, Talk, Book and Pen build
// what they give as the settings are now, on the interface's thread, and
// fail at once when the settings name no model or no key; what they give
// sends, off that thread.
type Service interface {
	// ConfigFile is where the services are configured, or empty when there
	// is no such file.
	ConfigFile() string
	// Providers are the services configured, in their order.
	Providers() []Provider
	// Reload reads the configuration again; when that fails, the services
	// stay as they were.
	Reload() error
	// SaveProvider saves p in place of the service with its ID, or after
	// the others when none has it.
	SaveProvider(p Provider) error
	// SetKey keeps the API key of a service in the system's keychain;
	// empty removes it.
	SetKey(providerID, key string) error
	// Catalog lists the models of a service.
	Catalog(providerID string) (Catalog, error)
	// Efforts are the reasoning efforts offered, in order; EffortLabel
	// names one.
	Efforts() []string
	EffortLabel(effort string) string

	// Talk talks over the work of s with its discussion model, sending
	// what the author is writing beside when it is not nil; progress
	// hears the reply as it streams, off the interface's thread.
	Talk(s *workspace.Store, beside *Beside, progress func(reasoning, text string)) (Talk, error)
	// Extract takes the words [start, end) of a message of the talk into
	// the setting under category.
	Extract(s *workspace.Store, session, message string, start, end int, category string) (workspace.Entry, error)
	// Bookkeep starts bringing the setting of s up to date in the
	// background; changed hears each revision written, and the work
	// starting and stopping, off the interface's thread.
	Bookkeep(s *workspace.Store, changed func()) Bookkeeping
	// Book is the bookkeeper of s, for a batch to be tried again.
	Book(s *workspace.Store) (Book, error)
	// Pen writes the prose of s with its writing model.
	Pen(s *workspace.Store) (Pen, error)
	// Unsummarized is the documents before chapter long enough for a
	// summary that have none, in the order of the tree, and how many
	// before it have one.
	Unsummarized(s *workspace.Store, chapter string) (missing []string, have int, err error)
	// MinSummary and MinCheck are the shortest prose worth a summary and
	// a check.
	MinSummary() int
	MinCheck() int
}

// Provider is a service of models. Its key is kept apart, by SetKey.
type Provider struct {
	ID       string
	Name     string
	BaseURL  string
	Protocol string
	Env      string // the environment variable holding the key, if any
	Models   []string
}

// Catalog lists the models a service offers.
type Catalog interface {
	ListModels(ctx context.Context) ([]string, error)
}

// Talk is the discussion of one work.
type Talk interface {
	// Say saves the author's words, then asks for a reply. When that
	// fails, saved tells whether the words were kept, for Retry.
	Say(ctx context.Context, session string, words workspace.Event) (saved bool, err error)
	// Retry asks again for the reply to the author's words userID.
	Retry(ctx context.Context, session, userID string) error
}

// Beside is what the author is writing as they talk beside the prose: a
// chapter, or a volume when a folder is open.
type Beside struct {
	Title, Outline string // the chapter's, or the volume's
	Volume         string // the title of the volume a chapter lies in
	VolumeOutline  string
	Folder         bool
	Before         string // the chapter's text before the caret
}

// Bookkeeping is the bookkeeper of one work running in the background.
type Bookkeeping interface {
	// Kick has it look for turns to take in.
	Kick()
	// Busy reports whether it is taking turns in, retries and the waits
	// before them included.
	Busy() bool
	// Stop ends it: what it has not taken in waits for the next start.
	Stop()
}

// Book is the bookkeeper of one work.
type Book interface {
	// Redo tries again, once, the turns a failed revision covered.
	Redo(ctx context.Context, failed string) error
}

// Pen writes the prose of one work with one model.
type Pen interface {
	// Name is the service and the model, as provider/model.
	Name() string
	Effort() string
	// Predict continues the chapter of scene with one sentence.
	Predict(ctx context.Context, scene Scene) (string, error)
	// Rewrite prepares rewriting selected, the words between the scene's
	// before and after, as the instruction asks.
	Rewrite(scene Scene, selected, instruction string) (Job, error)
	// Draft prepares drafting the chapter of scene from its outline, into
	// about words characters, or as many as the outline wants for 0.
	Draft(scene Scene, words int) (Job, error)
	// Summarize sums up the chapter as it is on disk.
	Summarize(ctx context.Context, chapter string) (workspace.Summary, error)
	// Check reads the chapter as it is on disk against the setting.
	Check(ctx context.Context, chapter string) (workspace.Check, error)
}

// Scene is where in a chapter the model writes: the text before that
// place and after it, as the editor has them now.
type Scene struct {
	Chapter       string
	Before, After string
}

// Job is a rewrite or a draft ready to run.
type Job struct {
	// Record is what the snapshot taken before keeps of it.
	Record workspace.AIRecord
	// Run sends the request, passing the prose to onText as it streams,
	// and returns all of it; what came before a failure is returned with
	// the error.
	Run func(ctx context.Context, onText func(string)) (string, error)
}
