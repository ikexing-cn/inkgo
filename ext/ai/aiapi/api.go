// Package aiapi is the port between the interface and the AI: the
// services and recorded model access this machine has, and the pen of a work.
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

// Assistant selects recorded model handles and builds a work's pen on the
// frame thread. Both fail for invalid settings or while AI is off; completions
// and the pen send off that thread.
type Assistant interface {
	// Model selects a role on the frame thread. Its completer runs off that
	// thread and preserves usage records, traces and activation cancellation.
	Model(*workspace.Store, Role) (Handle, error)
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

// Service resolves the active machine backend.
const Service ext.ServiceKey[Backend] = "ai.service"

// ToolEnv is the immutable environment of a tool call off the UI thread.
type ToolEnv struct {
	Store         *workspace.Store
	Session, Turn string
	// Call identifies this tool invocation for its owner.
	Call          string
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
