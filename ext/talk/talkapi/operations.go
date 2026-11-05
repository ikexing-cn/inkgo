package talkapi

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/workspace"
)

// ProposalAddress identifies a proposal in a conversation.
type ProposalAddress struct{ Session, Call string }

// StartTalk is a frame-thread request to start a conversation.
type StartTalk struct {
	Services ui.Services
	Event    workspace.Event
}

// DeclareOperations declares the typed state-changing conversation collaboration calls.
func DeclareOperations(r *ext.Registrar) {
	ext.DeclareOperation(r, Proposals, "Begin", InvokeBegin)
	ext.DeclareOperation(r, Proposals, "Follow", InvokeFollow)
	ext.DeclareOperation(r, Proposals, "Finish", InvokeFinish)
	ext.DeclareOperation(r, Proposals, "Drop", InvokeDrop)
	ext.DeclareOperation(r, Proposals, "ShowProposal", InvokeShowProposal)
	ext.DeclareOperation(r, TalkStarters, "Start", InvokeStart)
	ext.DeclareOperation(r, TalkStarters, "Open", InvokeOpen)
}

// InvokeBegin adapts the Begin proposal operation.
func InvokeBegin(p Suggestions) func(struct{}) struct{} {
	return func(in struct{}) struct{} { p.Begin(); return struct{}{} }
}

// InvokeFollow adapts the Follow proposal operation.
func InvokeFollow(p Suggestions) func(ProposalFrame) struct{} {
	return func(in ProposalFrame) struct{} { p.Follow(in); return struct{}{} }
}

// InvokeFinish adapts the Finish proposal operation.
func InvokeFinish(p Suggestions) func(string) struct{} {
	return func(in string) struct{} { p.Finish(in); return struct{}{} }
}

// InvokeDrop adapts the Drop proposal operation.
func InvokeDrop(p Suggestions) func(struct{}) struct{} {
	return func(in struct{}) struct{} { p.Drop(); return struct{}{} }
}

// InvokeShowProposal adapts the ShowProposal proposal operation.
func InvokeShowProposal(p Suggestions) func(ProposalAddress) struct{} {
	return func(in ProposalAddress) struct{} { p.ShowProposal(in.Session, in.Call); return struct{}{} }
}

// InvokeStart adapts conversation creation.
func InvokeStart(p TalkStarter) func(StartTalk) bool {
	return func(in StartTalk) bool { return p.Start(in.Services, in.Event) }
}

// InvokeOpen adapts conversation navigation.
func InvokeOpen(p TalkStarter) func(string) struct{} {
	return func(id string) struct{} { p.Open(id); return struct{}{} }
}
