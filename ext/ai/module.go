// Package ai owns the machine AI backend and its settings.
package ai

import (
	"context"
	"github.com/egoist/mygo/ui"
	core "writer/ai"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/ai/aitext"
	"writer/ext/hostapi"
	"writer/model"
)

// Options supplies the configuration location, trace destination and an optional
// backend factory for tests. Factory runs only at activation and retains
// ownership of injected backends; the default backend is closed on Stop.
type ModelDataState = model.ModelUpdateState
type ModelUpdater interface {
	Status() ModelDataState
	Sync(context.Context) error
	SetEnabled(bool)
	OnChange(func())
	Close()
}
type Options struct {
	ConfigFile, Trace string
	Factory           func() aiapi.Backend
	View              *View
	Updates           ModelUpdater
}

// ConfigPath locates this machine's model configuration.
func ConfigPath() (string, error) { return core.ConfigPath() }

// Module registers AI using the default configuration location.
var Module = New(Options{})

// View owns the settings draft for one assembly, on the frame thread.
type View struct {
	modelsAuto    bool
	modelsSyncing bool
	autoModels    *ext.Pref[bool]
	updates       ModelUpdater
	ai            aiapi.Backend
	ServiceForm   ServiceForm
	book          *ext.Ref[hostapi.Book]
	editing       *ext.Ref[hostapi.Editing]
	palette       *ext.Ref[hostapi.Palette]
	jobs          *ext.Ref[hostapi.Jobs]
	modelSections *ext.PointReader[hostapi.SettingsSection]
	modelChanges  *ext.PointReader[func()]
	usageItems    *ext.PointReader[func(*ui.Context)]
	pal           *kit.Palette
	status        *kit.Status
}

// New constructs the prerequisite without IO during Setup.
func New(opts Options) ext.Module {
	return ext.Module{ID: "ai", Kind: ext.Library, Group: "lib", Name: aitext.ModAI, Desc: aitext.ModAIAbout, Setup: func(r *ext.Registrar) {
		v := opts.View
		if v == nil {
			v = &View{}
		}
		v.autoModels = ext.Setting(r, "models-auto", ext.Schema[bool]{Title: aitext.SettingsModelsAuto, Default: true, Scope: ext.Local, Drawn: true})
		v.updates = opts.Updates
		v.book = ext.Use(r, hostapi.BookService)
		v.editing = ext.Use(r, hostapi.EditingService)
		v.palette = ext.Use(r, hostapi.PaletteService)
		v.jobs = ext.Use(r, hostapi.JobsService)
		notices := ext.Use(r, hostapi.NoticesService)
		ext.Declare(r, aiapi.ModelChanges, ext.Hidden)
		ext.DeclareCalls(r, aiapi.ModelChanges, func(f func()) func(struct{}) struct{} { return func(struct{}) struct{} { f(); return struct{}{} } })
		v.modelChanges = ext.ReadPoint(r, aiapi.ModelChanges)
		ext.Declare(r, aiapi.Tools, ext.Hidden) // tools the model calls, not buttons
		ext.DeclareCalls(r, aiapi.Tools, aiapi.InvokeTool)
		ext.Declare(r, aiapi.Usage, "usage")
		ext.DeclareCalls(r, aiapi.Usage, func(draw func(*ui.Context)) func(*ui.Context) struct{} {
			return func(c *ui.Context) struct{} { draw(c); return struct{}{} }
		})
		ext.Declare(r, aiapi.Calls, ext.Hidden)
		ext.DeclareCalls(r, aiapi.Calls, func(f func(aiapi.Call)) func(aiapi.Call) struct{} {
			return func(c aiapi.Call) struct{} { f(c); return struct{}{} }
		})
		calls := ext.ReadPoint(r, aiapi.Calls)
		v.usageItems = ext.ReadPoint(r, aiapi.Usage)
		v.modelSections = ext.ReadPoint(r, hostapi.SettingsSections)
		ext.Provide(r, aiapi.Service, func(activation *ext.Session) aiapi.Backend {
			if opts.Factory != nil {
				v.ai = opts.Factory()
				for {
					source, ok := v.ai.(interface{ BackendSource() aiapi.Backend })
					if !ok {
						break
					}
					v.ai = source.BackendSource()
				}
			} else {
				backend := core.New(opts.ConfigFile)
				backend.Trace = opts.Trace

				backend.ErrorText = func(err error) string {
					if text, ok := aitext.Text(err); ok {
						return text
					}
					return err.Error()
				}
				v.ai = backend
			}
			backend := accountingBackend{v.ai, calls, activation}
			backend.prepare()
			return backend
		})
		r.OnStart(func(*ext.Session) error {
			v.modelsAuto = v.autoModels.Get()
			v.pal = v.palette.Get().Colors()
			v.status = notices.Get().Notices()
			if v.updates == nil && opts.Factory == nil && v.ai.ConfigFile() != "" {
				v.updates = model.NewModelUpdates(v.ai.ConfigFile(), "", nil)
			}
			if v.updates != nil {
				v.modelsAuto = v.autoModels.Get()
				v.updates.SetEnabled(v.modelsAuto)
			}
			v.ai.BindSwitch(func() bool { return v.ai != nil })
			if v.ai.ConfigFile() != "" {
				if err := v.ai.Reload(); err != nil {
					v.status.SetError(err)
				}
			}
			return nil
		})
		r.OnStop(func(*ext.Session) {
			if closer, ok := v.ai.(interface{ Close() }); ok && opts.Factory == nil {
				closer.Close()
			}
			if v.updates != nil {
				v.updates.Close()
				v.updates = nil
			}
			v.ai = nil
			v.ServiceForm.Close()
		})
		ext.Contribute(r, hostapi.ErrorTexts, "errors", hostapi.ErrorTranslator(func(err error, reason func(error) string) (string, bool) {
			if text, ok := formErrorText(err, reason); ok {
				return text, true
			}
			return aitext.Translate(err, reason)
		}))
		ext.Contribute(r, hostapi.SettingsPages, "settings", hostapi.SettingsPage{Title: aitext.ModAI, Label: "设置-AI", Order: 10})
		ext.Contribute(r, hostapi.SettingsSections, "settings-content", hostapi.SettingsSection{Draw: func(c *ui.Context) { v.pal = v.palette.Get().Colors(); v.modelsView(c) }})
	}}
}

// SetBackend replaces the settings backend in integration fixtures on the frame thread.
func (v *View) SetBackend(backend aiapi.Backend) { v.ai = backend }

func (v *View) SetModelUpdater(u ModelUpdater) { v.updates = u }
