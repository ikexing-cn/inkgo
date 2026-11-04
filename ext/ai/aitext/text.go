// Package aitext translates AI errors into interface messages. The lower-level
// model, writing and inference packages do not produce UI copy;
// this package is the extension presentation layer, registered through hostapi.
package aitext

import (
	"reflect"
	"writer/locales"

	"writer/ai"
	"writer/inference"
	"writer/model"
	"writer/tokens"
	"writer/writing"
)

// Text translates known AI errors in the current locale. Unknown errors return false.
func Text(err error) (string, bool) { return Translate(err, text) }

func text(err error) string {
	if err == nil {
		return ""
	}
	if s, ok := Text(err); ok {
		return s
	}
	return err.Error()
}

// Translate uses reason for nested causes, preserving the host's translator chain.
func Translate(err error, reason func(error) string) (string, bool) {
	if reason == nil {
		reason = text
	}
	switch cause := err.(type) {
	case *model.SetupError:
		return reason(cause.Err), true
	case *model.HTTPError:
		switch cause.Code {
		case 401, 403:
			return StatusHTTPAuth(cause.Code), true
		case 404:
			return StatusHTTPMissing(cause.Code), true
		case 429:
			return StatusHTTPRate(cause.Code), true
		case 400, 422:
			return StatusHTTPRejected(cause.Code), true
		default:
			if cause.Code >= 500 {
				return StatusHTTPServer(cause.Code), true
			}
		}
		return StatusHTTPOther(cause.Code), true
	case *writing.CheckResultError:
		return CheckResultError(reason(cause.Err)), true
	case *writing.ShortChapterError:
		return CheckShortChapter(cause.Count, cause.Unit), true
	case *inference.OutputLimitError:
		return StatusOutputLimit(tokens.Format(cause.Limit)), true
	case *inference.JSONError:
		return StatusJSONError(reason(cause.Err)), true
	case *model.AbortedError:
		return StatusModelAborted(reason(cause.Err)), true
	case *model.TimeoutError:
		return StatusModelTimeout(reason(cause.Err)), true
	case *model.ToolError:
		return ChatToolError(cause.Name), true
	case *model.EffortError:
		return SettingsEffortError(cause.Effort), true
	case *model.ProviderMissingError:
		return SettingsProviderMissing(cause.ID), true
	case *model.ProviderError:
		return SettingsProviderError(cause.Count, reason(cause.Err)), true
	case *model.ConfigError:
		switch cause.Op {
		case "locate":
			return SettingsConfigLocate(reason(cause.Err)), true
		case "open":
			return SettingsConfigOpen(reason(cause.Err)), true
		case "read":
			return SettingsConfigRead(reason(cause.Err)), true
		case "mkdir":
			return SettingsConfigMkdir(reason(cause.Err)), true
		case "create":
			return SettingsConfigCreate(reason(cause.Err)), true
		case "replace":
			return SettingsConfigReplace(reason(cause.Err)), true
		case "write":
			return SettingsConfigWrite(reason(cause.Err)), true
		}
	case *model.ContextLimitError:
		switch cause.Part {
		case "outline":
			return StatusContextOutline(), true
		case "chapter":
			return StatusContextChapter(), true
		case "lore":
			return StatusContextLore(), true
		case "turn":
			return StatusContextTurn(), true
		case "message":
			return StatusContextMessage(), true
		case "index":
			return StatusContextIndex(), true
		case "summary":
			return StatusContextSummary(), true
		case "attachments":
			return StatusContextAttachments(), true
		}
	case *model.LimitsError:
		switch cause.Rule {
		case "integer":
			return SettingsModelAtInt(cause.ID), true
		case "numbers":
			return SettingsModelNumbers(cause.ID), true
		case "output":
			return SettingsModelOutMax(cause.ID), true
		case "threshold":
			return SettingsModelAtRange(cause.ID), true
		case "duplicate":
			return SettingsModelDup(cause.ID), true
		}

	}
	if err != nil && reflect.TypeOf(err).Comparable() {
		if label, ok := errorLabels[err]; ok {
			return label(), true
		}
	}
	return "", false
}

var errorLabels = map[error]func() string{
	model.ErrRequestTooLarge:   StatusRequestTooLarge,
	model.ErrTruncated:         StatusTruncated,
	model.ErrKeyNewline:        SettingsKeyNewline,
	model.ErrRedirect:          StatusRedirect,
	model.ErrModelRequired:     SettingsModelRequired,
	model.ErrNegativeTokens:    StatusNegativeTokens,
	model.ErrMessagesRequired:  StatusMessagesRequired,
	model.ErrToolJSON:          StatusToolJSON,
	model.ErrDisconnected:      StatusDisconnected,
	model.ErrFiltered:          StatusFiltered,
	model.ErrIncomplete:        StatusIncomplete,
	model.ErrReplyTooLarge:     StatusReplyTooLarge,
	model.ErrNoText:            StatusNoText,
	model.ErrToolCallMissing:   StatusToolCallMissing,
	model.ErrRole:              StatusRole,
	model.ErrService:           StatusService,
	model.ErrConnection:        StatusConnection,
	model.ErrDestination:       StatusDestination,
	model.ErrConfigTooLarge:    SettingsConfigTooLarge,
	model.ErrConfigYAML:        SettingsConfigYAML,
	model.ErrConfigDocuments:   SettingsConfigDocuments,
	model.ErrConfigEncode:      SettingsConfigEncode,
	model.ErrIDFormat:          SettingsIDRule,
	model.ErrConfigVersion:     SettingsConfigVersion,
	model.ErrProviderDuplicate: SettingsProviderDuplicate,
	model.ErrNameRequired:      SettingsNameRequired,
	model.ErrProtocol:          SettingsProtocolInvalid,
	model.ErrEnvFormat:         SettingsEnvRule,
	model.ErrURLScheme:         SettingsURLScheme,
	model.ErrURLHost:           SettingsURLNoHost,
	model.ErrURLExtra:          SettingsURLExtra,
	model.ErrModelID:           SettingsModelIDInvalid,
	model.ErrCatalogTotal:      SettingsCatalogTotal,
	model.ErrCatalogJSON:       SettingsCatalogJSON,
	model.ErrCatalogService:    SettingsCatalogService,
	model.ErrCatalogData:       SettingsCatalogData,
	model.ErrCatalogPagination: SettingsCatalogPagination,
	model.ErrCatalogPages:      SettingsCatalogPages,
	model.ErrRequestCreate:     StatusRequestCreate,
	model.ErrCatalogPageSize:   SettingsCatalogPageSize,
	model.ErrKeyDelete:         SettingsKeyDelete,
	model.ErrKeySave:           SettingsKeySave,
	model.ErrKeychain:          SettingsKeychain,
	ai.ErrOff:                  ChatOff,
	ai.ErrChatModel:            locales.ChatNeedModel,
	inference.ErrTrailingJSON:  StatusTrailingJSON,
	writing.ErrNoModelReason:   EditorWhyPickModel,
	writing.ErrChapterMissing:  StatusChapterMissing,
	writing.ErrNoSetting:       CheckNoSetting,
	model.ErrModelFields:       SettingsModelFields,
}
