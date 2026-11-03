// Package ext records extension declarations and manages their active sessions.
// It performs no filesystem IO; the application owns persistence.
package ext

import "context"

// Kind distinguishes user features from shared libraries.
type Kind uint8

const (
	Feature Kind = iota // Feature can have a book-specific switch.
	Library             // Library always follows its local switch.
)

// HostOwner is reserved for points declared by the application in a later layer.
const HostOwner = "host"

// Module describes an extension. Setup only records declarations, once per Host.
type Module struct {
	ID                 string
	Kind               Kind
	Name, Desc         func() string
	Requires, Optional []string
	Setup              func(*Registrar)
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

// SettingStore persists typed values by prefixed setting ID. A zero Book means
// local storage. Calls are serialized by Host; Save must finish before returning.
type SettingStore interface {
	Load(Book, string) (any, bool)
	Save(Book, string, any)
}

// Options supplies storage and the directory containing local extension data.
// Nil stores use host-owned memory. Saves commit the local layer first; if a
// subsequent book save fails, that successful local change remains effective.
type Options struct {
	LocalRoot     string
	LocalSwitches LocalSwitchStore
	BookSwitches  BookSwitchStore
	Settings      SettingStore
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

// Item is an active contribution, in assembly and registration order.
type Item[T any] struct {
	ID, Owner string
	Value     T
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
}

// InjectionKind reserves the vocabulary for the next implementation stage.
type InjectionKind uint8

const (
	Before InjectionKind = iota
	After
	Around
	Overwrite
	Replace
)

// Injection reserves ledger space; registration and execution are not yet offered.
type Injection struct {
	ID, Target       string
	Kind             InjectionKind
	Order            int
	Active, Conflict bool
}

// Failure retains the original error and the lifecycle location.
type Failure struct {
	Stage string
	Err   error
}

// Ledger is a snapshot of all declarations, including those of inactive modules.
type Ledger struct {
	ID                         string
	Requires, Optional         []Dependency
	NeededBy, UsedOptionallyBy []string
	Points                     []string
	Contributions              []Contribution
	Provides                   []string
	Uses                       []ServiceUse
	Settings                   []SettingInfo
	Injections                 []Injection
	State                      State
	Errors                     []Failure
}

// Task is a background operation tied to a session's cancellation and shutdown.
type Task = func(context.Context)
