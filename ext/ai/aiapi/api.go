// Package aiapi is the port between the interface and the AI: the
// services of models this machine has, and the talk and the pen of a work.
// Package desk uses the AI through it alone, so that desk imports nothing
// that sends; package ai fills it.
package aiapi

import (
	"context"
	"encoding/json"
	"writer/ext"

	"writer/workspace"
)

// SetupError means that the service, model or key can be corrected in settings.
// Err keeps its original text and remains available to errors.Is and errors.As.
type SetupError struct{ Err error }

// Error returns the nested error text.
func (e *SetupError) Error() string { return e.Err.Error() }

// Unwrap preserves the underlying error identity.
func (e *SetupError) Unwrap() error { return e.Err }

// Setup marks failures that can be corrected in AI settings.
func (e *SetupError) Setup() {}

// Settings configures this machine's model services. Catalog builds on
// the interface's thread and fails at once for invalid settings or while
// AI is off; the returned catalog sends off that thread.
type Settings interface {
	// BindSwitch supplies the machine's switch. The function is read only
	// while Catalog, Talk or Pen builds on the interface's thread.
	BindSwitch(on func() bool)
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
	// Check says what keeps p from being saved, by the field each problem
	// is in, so that a form can say it there before it is pressed.
	Check(p Provider) Problems
	// Limits are the limits model id starts with when it is added to a
	// service: as the table built in has them, when known says it has the
	// model, or careful ones.
	Limits(id string) (m Model, known bool)
	// SetKey keeps the API key of a service in the system's keychain;
	// empty removes it.
	SetKey(providerID, key string) error
	// Catalog lists the models of a service.
	Catalog(providerID string) (Catalog, error)
	// Efforts are the reasoning efforts offered, in order; EffortLabel
	// names one.
	Efforts() []string
	EffortLabel(effort string) string
	// Cost is what a request that took and gave u costs on the model
	// named provider/model, by the prices built in, in currency; known is
	// false for a model with no price.
	Cost(model string, u workspace.Usage) (cost float64, currency string, known bool)
}

// Assistant builds a work's talk and pen on the interface's thread,
// failing at once for invalid settings or while AI is off. The returned
// talk and pen send off that thread.
type Assistant interface {
	// Talk talks over the work of s with its discussion model, sending
	// what the author is writing beside when it is not nil, unless the
	// talk keeps to the chapter it was opened to draft or rewrite. Off the
	// interface's thread, progress hears the reply as it streams, and step
	// each event it writes on the way: the calls of a step to look things
	// up, note the setting or propose a change of the prose, and what each
	// returned.
	Talk(s *workspace.Store, material Material, tools []Tool, beside *Beside, progress func(Progress), step func(workspace.Event)) (Talk, error)
	// Preloaded describes what a talk opened to draft or rewrite chapter
	// of s carries before anything is said or looked up: the tokens, the
	// chapter's cards, whether the chapter before goes whole, and where
	// the outlines come from.
	Preloaded(s *workspace.Store, material Material, origin, chapter string) (Preload, error)
	// Extract takes the words [start, end) of a message of the talk into
	// the setting under category.
	Extract(s *workspace.Store, session, message string, start, end int, category string) (workspace.Entry, error)
	// Pen writes the prose of s with its writing model.
	Pen(s *workspace.Store, material Material) (Pen, error)
	// MinCheck is the shortest prose worth a check.
	MinCheck() int
}

// Backend combines Settings and Assistant so desk holds one value for
// the same service that fills both.
type Backend interface {
	Settings
	Assistant
}

// OutlineSource says where the outlines a talk carries before its chapter
// come from.
type OutlineSource int

const (
	// BookVolumes is the outlines of the book's volumes.
	BookVolumes OutlineSource = iota
	// VolumeChapters is the outlines of the chapters before this one in its volume.
	VolumeChapters
)

// Preload describes a talk's fixed context: the tokens it takes, the
// chapter's cards carried, whether the chapter before goes whole, and where
// the outlines come from, even when the outlines are all empty.
type Preload struct {
	Tokens, Cards int
	Previous      bool
	Outlines      OutlineSource
}

// Provider is a service of models. Its key is kept apart, by SetKey.
type Provider struct {
	ID       string
	Name     string
	BaseURL  string
	Protocol string
	Env      string // the environment variable holding the key, if any
	Models   []Model
}

// Problems are what keep a service from being saved, by the field each is
// in; a field with none has nil.
type Problems struct {
	ID, Name, Protocol, BaseURL, Env, Models error
}

// First is the first problem in the order of the form, or nil.
func (p Problems) First() error {
	for _, s := range []error{p.ID, p.Name, p.Protocol, p.BaseURL, p.Env, p.Models} {
		if s != nil {
			return s
		}
	}
	return nil
}

// Model is a model of a service and its limits, in tokens: a request and
// its reply together take at most Window of them, the reply at most
// Output; a talk is compacted at CompactAt percent of Window.
type Model struct {
	ID                        string
	Window, Output, CompactAt int
}

// Catalog lists a provider’s models off the UI thread.
type Catalog interface {
	ListModels(ctx context.Context) ([]string, error)
}

// Progress is a piece of a reply as it streams: of its reasoning or its
// text, or a change of the prose it proposes as far as it came, by the
// call that proposes it.
type Progress struct {
	Reasoning, Text string
	Call            string
	Proposal        *workspace.Proposal
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

// Pen writes the prose of one work with one model.
type Pen interface {
	// Name is the service and the model, as provider/model.
	Name() string
	Effort() string
	// Predict continues the chapter of scene with one sentence.
	Predict(ctx context.Context, scene Scene) (string, error)
	// Check reads the chapter as it is on disk against the setting.
	Check(ctx context.Context, chapter string) (workspace.Check, error)
}

// Scene is where in a chapter the model writes: the text before that
// place and after it, as the editor has them now.
type Scene struct {
	Chapter       string
	Before, After string
}

// Material identifies reference providers available to a request. False means
// no provider, not empty content. The zero value has no providers.
type Material struct {
	Setting, Outline bool
}

// NoOutlines means no outline provider contributed fixed context.
const NoOutlines OutlineSource = -1

// Service resolves the active machine backend.
const Service ext.ServiceKey[Backend] = "ai.service"

// ToolEnv is the immutable environment of a tool call off the UI thread.
type ToolEnv struct {
	Store         *workspace.Store
	Session, Turn string
	At            string
	Model, Effort string
	Shared        *ToolShared
}

// ToolResult reports a call's output and changes.
type ToolResult struct {
	Text     string
	Failed   bool
	Revision string
	Proposal *workspace.Proposal
}

// Tool runs and labels calls off the UI thread, without per-frame objects.
type Tool struct {
	Name, Description string
	// Order is the position in the tool list sent to the model.
	// Ties retain registration order.
	Order  int
	Schema func(workspace.Work) json.RawMessage
	Run    func(context.Context, ToolEnv, json.RawMessage) ToolResult
	Label  func(ToolEnv, json.RawMessage) string
	Prompt ToolPrompt
}

// ToolPrompt supplies lookup phrases, notes and sections for the system prompt.
type ToolPrompt struct {
	// Order is the position in the lookup sentence and among prompt sections.
	Order                  int
	Lookup, Notes, Section string
	// SettingIndex follows the generic introduction to a setting index.
	SettingIndex string
}

// Tools collects tools contributed to talk by active extensions.
const Tools ext.PointKey[Tool] = "ai.tools"
