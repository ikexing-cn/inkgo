// Package ext records extension declarations and manages their active sessions.
// It performs no filesystem IO; the application owns persistence.
package ext

import (
	"context"
	"time"
)

// Kind distinguishes user features from shared libraries.
type Kind uint8

const (
	Feature Kind = iota // Feature can have a book-specific switch.
	Library             // Library always follows its local switch.
)

// HostOwner is reserved for points and hooks declared by the application.
const HostOwner = "host"

// Module describes an extension. Setup only records declarations, once per Host.
type Module struct {
	ID string
	// Installed distinguishes separately installed modules from bundled modules.
	Installed bool
	Kind      Kind
	// Name and Desc are localization only and remain safe while disabled.
	Name, Desc func() string
	// Optional dependencies must be installed; only their activation is optional.
	Requires, Optional []string
	// Group is a host display grouping key; ext does not interpret it.
	Group string
	// Off supplies an extra host display sentence when the own switch is off.
	// Nil says nothing; ext does not interpret it.
	Off   func() string
	Setup func(*Registrar)
}

// PointKey names a contribution point without holding runtime state.
type PointKey[T any] string

// ServiceKey names a session service without holding runtime state.
type ServiceKey[T any] string

// Book identifies the open book; an empty Dir represents the shelf.
type Book struct{ Dir string }

// LocalSwitchStore persists the IDs whose local switches are off.
type LocalSwitchStore interface {
	Load() ([]string, error)
	Save(off []string) error
}

// BookSwitchStore persists explicit overrides, keyed by book and module ID.
type BookSwitchStore interface {
	Load(Book) (map[string]bool, error)
	Save(Book, map[string]bool) error
}

// SettingStore persists JSON bytes by prefixed setting ID. A zero Book means
// local storage. Calls are serialized by Host; Save must finish before returning.
type SettingStore interface {
	Load(Book, string) ([]byte, bool)
	Save(Book, string, []byte)
}

// Options supplies storage and the directory containing local extension data.
// Nil stores use host-owned memory. Saves commit the local layer first; if a
// subsequent book save fails, that successful local change remains effective.
type Options struct {
	// BookDir resolves per-book extension directories through the workspace.
	// Nil makes Session.BookDir unavailable.
	BookDir       func(dir, id string) (string, error)
	LocalRoot     string
	LocalSwitches LocalSwitchStore
	BookSwitches  BookSwitchStore
	Settings      SettingStore
	// BookState stores module-owned book preferences.
	BookState BookStateStore
	// DeclareHost records host-owned points, hooks and services before module Setup.
	DeclareHost func(*Registrar)
	// HostKeys are the host's own key bindings. They take part in key claims
	// and always win; their claims are in Ledger(HostOwner).Keys.
	HostKeys []HostKey
	// Injections persists author-controlled ordering and choices.
	Injections InjectionStore
	// OnFailure reports failures without the state lock. It can run on any calling
	// goroutine; marshal notifications to the UI and do not mutate Host here.
	OnFailure func(id string, f Failure)
}

// Scope selects the persistence layer of a preference.
type Scope uint8

const (
	Local     Scope = iota // Local uses the same value in every book.
	BookScope              // BookScope uses the book value, or its default on the shelf.
	Both                   // Both uses a book override, falling back to local.
)

// Choice describes an offered value; values are not restricted to this list.
type Choice[T any] struct {
	Value T
	Desc  func() string
}

// Schema describes a preference for host-rendered settings.
type Schema[T any] struct {
	Title, Desc func() string
	Default     T
	Scope       Scope
	Choices     []Choice[T]
	Group       string
	Order       int
}

// State reports the lifetime of a module's current session.
type State uint8

const (
	Off State = iota // Off includes modules blocked by a required dependency.
	Starting
	On
	Failed
)

// Item describes a contribution, in assembly and registration order.
type Item[T any] struct {
	ID, Owner string
	Value     T
	// ReplacedBy is the supplying injection ID, or empty for the original.
	ReplacedBy string
}

// Dependency reports whether a declared dependency is currently active.
type Dependency struct {
	ID     string
	Active bool
}

// Contribution records a contribution even when either endpoint is inactive.
type Contribution struct{ ID, Point string }

// ServiceUse records whether a service was requested as optional.
type ServiceUse struct {
	Key      string
	Optional bool
}

// SettingInfo retains schema metadata without erasing it to a single value type.
// Schema contains the original Schema[T], including choices and default.
type SettingInfo struct {
	ID          string
	Title, Desc func() string
	Scope       Scope
	Group       string
	Order       int
	Schema      any
	// Read and Write access the declared value on the frame thread, outside Setup.
	Read  func() any
	Write func(any)
}

// InjectionKind identifies an interception operation.
type InjectionKind uint8

const (
	BeforeKind InjectionKind = iota
	AfterKind
	AroundKind
	OverwriteKind
	ReplaceKind
)

// ReasonKind identifies an injection status without prescribing display text.
type ReasonKind uint8

const (
	ReasonEnabled     ReasonKind = iota // ReasonEnabled has no related ID.
	ReasonInjectorOff                   // ReasonInjectorOff identifies the inactive injector.
	ReasonTargetOff                     // ReasonTargetOff identifies the inactive target or point owner.
	ReasonConflict                      // ReasonConflict has no single related ID; inspect the target's chain.
	ReasonNotSelected                   // ReasonNotSelected identifies the selected injection.
	ReasonReplaced                      // ReasonReplaced identifies the replacing injection.
)

// Reason supplies a status kind and a related module or injection ID for the UI
// to translate. ID is empty for ReasonEnabled and ReasonConflict.
type Reason struct {
	Kind ReasonKind
	ID   string
}

// Injection describes an interception, including inactive and conflicting entries.
type Injection struct {
	ID, Target       string
	Kind             InjectionKind
	Order            int
	Active, Conflict bool
	Owner            string
	Reason           Reason
}

// Failure retains the original error and the lifecycle location.
type Failure struct {
	// At records when the failure occurred.
	At    time.Time
	Stage string
	Err   error
	// Module, Target, Injection and Chain locate the failing callback.
	Module, Target, Injection, Chain string
}

// Ledger is a snapshot of all declarations, including those of inactive modules.
// CallInfo records a point operation and its input/output type names at assembly.
type CallInfo struct{ Point, Operation, Input, Output string }

// Ledger is a snapshot of declarations and runtime status.
type Ledger struct {
	// Calls records the typed operations declared by this module.
	Calls                      []CallInfo
	ID                         string
	Requires, Optional         []Dependency
	NeededBy, UsedOptionallyBy []string
	Points                     []string
	// Reads records typed point consumption validated during assembly.
	Reads         []string
	Contributions []Contribution
	Provides      []string
	Uses          []ServiceUse
	Settings      []SettingInfo
	Injections    []Injection
	State         State
	Errors        []Failure
	Keys          []KeyClaim
}

// Task is a background operation tied to a session's cancellation and shutdown.
type Task = func(context.Context)
