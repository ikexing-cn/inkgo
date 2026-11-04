// Package ai owns the machine AI backend and its settings.
package ai

import (
	"github.com/egoist/mygo/ui"
	core "writer/ai"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/ai/aitext"
	"writer/ext/hostapi"
)

// Options supplies the configuration location, trace destination and an optional
// backend factory for tests. Factory runs only at activation and retains
// ownership of injected backends; the default backend is closed on Stop.
type Options struct {
	ConfigFile, Trace string
	Factory           func() aiapi.Backend
	View              *View
}

// ConfigPath locates this machine's model configuration.
func ConfigPath() (string, error) { return core.ConfigPath() }

// Module registers AI using the default configuration location.
var Module = New(Options{})

// View owns the settings draft for one assembly, on the frame thread.
type View struct {
	ai            aiapi.Backend
	ServiceForm   ServiceForm
	book          *ext.Ref[hostapi.Book]
	palette       *ext.Ref[hostapi.Palette]
	host          *ext.Ref[hostapi.SettingsHost]
	modelSections *ext.PointReader[hostapi.SettingsSection]
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
		v.book = ext.Use(r, hostapi.BookService)
		v.palette = ext.Use(r, hostapi.PaletteService)
		v.host = ext.Use(r, hostapi.SettingsHostService)
		notices := ext.Use(r, hostapi.NoticesService)
		ext.Declare(r, aiapi.Tools)
		ext.DeclareCalls(r, aiapi.Tools, aiapi.InvokeTool)
		v.usageItems = ext.ReadPoint(r, hostapi.AIUsage)
		v.modelSections = ext.ReadPoint(r, hostapi.SettingsSections)
		ext.Provide(r, aiapi.Service, func(*ext.Session) aiapi.Backend {
			if opts.Factory != nil {
				v.ai = opts.Factory()
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
			return v.ai
		})
		r.OnStart(func(*ext.Session) error {
			v.pal = v.palette.Get().Colors()
			v.status = notices.Get().Notices()
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
