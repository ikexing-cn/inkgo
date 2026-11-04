// Package aitext translates AI errors into interface messages. The lower-level
// model, discussion, writing and inference packages do not produce UI copy;
// this package is their presentation layer. When AI becomes an extension, this
// layer moves with it and registers its translators through hostapi.
package aitext

import (
	"reflect"

	"writer/ai"
	"writer/discussion"
	"writer/inference"
	"writer/locales"
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
			return locales.StatusHTTPAuth(cause.Code), true
		case 404:
			return locales.StatusHTTPMissing(cause.Code), true
		case 429:
			return locales.StatusHTTPRate(cause.Code), true
		case 400, 422:
			return locales.StatusHTTPRejected(cause.Code), true
		default:
			if cause.Code >= 500 {
				return locales.StatusHTTPServer(cause.Code), true
			}
		}
		return locales.StatusHTTPOther(cause.Code), true
	case *discussion.ReplyError:
		return locales.ChatReplyError(reason(cause.Err)), true
	case *discussion.CompactError:
		return locales.ChatCompactError(reason(cause.Err)), true
	case *writing.CheckResultError:
		return locales.CheckResultError(reason(cause.Err)), true
	case *writing.ShortChapterError:
		return locales.CheckShortChapter(cause.Count), true
	case *inference.OutputLimitError:
		return locales.StatusOutputLimit(tokens.Format(cause.Limit)), true
	case *inference.JSONError:
		return locales.StatusJSONError(reason(cause.Err)), true
	case *model.AbortedError:
		return locales.StatusModelAborted(reason(cause.Err)), true
	case *model.TimeoutError:
		return locales.StatusModelTimeout(reason(cause.Err)), true
	case *model.ToolError:
		return locales.ChatToolError(cause.Name), true
	case *model.EffortError:
		return locales.SettingsEffortError(cause.Effort), true
	case *model.ProviderMissingError:
		return locales.SettingsProviderMissing(cause.ID), true
	case *model.ProviderError:
		return locales.SettingsProviderError(cause.Count, reason(cause.Err)), true
	case *model.ConfigError:
		switch cause.Op {
		case "locate":
			return locales.SettingsConfigLocate(reason(cause.Err)), true
		case "open":
			return locales.SettingsConfigOpen(reason(cause.Err)), true
		case "read":
			return locales.SettingsConfigRead(reason(cause.Err)), true
		case "mkdir":
			return locales.SettingsConfigMkdir(reason(cause.Err)), true
		case "create":
			return locales.SettingsConfigCreate(reason(cause.Err)), true
		case "replace":
			return locales.SettingsConfigReplace(reason(cause.Err)), true
		case "write":
			return locales.SettingsConfigWrite(reason(cause.Err)), true
		}
	case *model.ContextLimitError:
		switch cause.Part {
		case "outline":
			return locales.StatusContextOutline(), true
		case "chapter":
			return locales.StatusContextChapter(), true
		case "lore":
			return locales.StatusContextLore(), true
		case "turn":
			return locales.StatusContextTurn(), true
		case "message":
			return locales.StatusContextMessage(), true
		case "index":
			return locales.StatusContextIndex(), true
		case "summary":
			return locales.StatusContextSummary(), true
		case "attachments":
			return locales.StatusContextAttachments(), true
		}
	case *model.LimitsError:
		switch cause.Rule {
		case "integer":
			return locales.SettingsModelAtInt(cause.ID), true
		case "numbers":
			return locales.SettingsModelNumbers(cause.ID), true
		case "output":
			return locales.SettingsModelOutMax(cause.ID), true
		case "threshold":
			return locales.SettingsModelAtRange(cause.ID), true
		case "duplicate":
			return locales.SettingsModelDup(cause.ID), true
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
	model.ErrRequestTooLarge:       locales.StatusRequestTooLarge,
	model.ErrTruncated:             locales.StatusTruncated,
	model.ErrKeyNewline:            locales.SettingsKeyNewline,
	model.ErrRedirect:              locales.StatusRedirect,
	model.ErrModelRequired:         locales.SettingsModelRequired,
	model.ErrNegativeTokens:        locales.StatusNegativeTokens,
	model.ErrMessagesRequired:      locales.StatusMessagesRequired,
	model.ErrToolJSON:              locales.StatusToolJSON,
	model.ErrDisconnected:          locales.StatusDisconnected,
	model.ErrFiltered:              locales.StatusFiltered,
	model.ErrIncomplete:            locales.StatusIncomplete,
	model.ErrReplyTooLarge:         locales.StatusReplyTooLarge,
	model.ErrNoText:                locales.StatusNoText,
	model.ErrToolCallMissing:       locales.StatusToolCallMissing,
	model.ErrRole:                  locales.StatusRole,
	model.ErrService:               locales.StatusService,
	model.ErrConnection:            locales.StatusConnection,
	model.ErrDestination:           locales.StatusDestination,
	model.ErrConfigTooLarge:        locales.SettingsConfigTooLarge,
	model.ErrConfigYAML:            locales.SettingsConfigYAML,
	model.ErrConfigDocuments:       locales.SettingsConfigDocuments,
	model.ErrConfigEncode:          locales.SettingsConfigEncode,
	model.ErrIDFormat:              locales.SettingsIDRule,
	model.ErrConfigVersion:         locales.SettingsConfigVersion,
	model.ErrProviderDuplicate:     locales.SettingsProviderDuplicate,
	model.ErrNameRequired:          locales.SettingsNameRequired,
	model.ErrProtocol:              locales.SettingsProtocolInvalid,
	model.ErrEnvFormat:             locales.SettingsEnvRule,
	model.ErrURLScheme:             locales.SettingsURLScheme,
	model.ErrURLHost:               locales.SettingsURLNoHost,
	model.ErrURLExtra:              locales.SettingsURLExtra,
	model.ErrModelID:               locales.SettingsModelIDInvalid,
	model.ErrCatalogTotal:          locales.SettingsCatalogTotal,
	model.ErrCatalogJSON:           locales.SettingsCatalogJSON,
	model.ErrCatalogService:        locales.SettingsCatalogService,
	model.ErrCatalogData:           locales.SettingsCatalogData,
	model.ErrCatalogPagination:     locales.SettingsCatalogPagination,
	model.ErrCatalogPages:          locales.SettingsCatalogPages,
	model.ErrRequestCreate:         locales.StatusRequestCreate,
	model.ErrCatalogPageSize:       locales.SettingsCatalogPageSize,
	model.ErrKeyDelete:             locales.SettingsKeyDelete,
	model.ErrKeySave:               locales.SettingsKeySave,
	model.ErrKeychain:              locales.SettingsKeychain,
	ai.ErrOff:                      locales.ChatOff,
	ai.ErrChatModel:                locales.ChatNeedModel,
	inference.ErrTrailingJSON:      locales.StatusTrailingJSON,
	writing.ErrNoModelReason:       locales.EditorWhyPickModel,
	writing.ErrChapterMissing:      locales.StatusChapterMissing,
	writing.ErrNoSetting:           locales.CheckNoSetting,
	discussion.ErrChapterGone:      locales.ChatChapterGone,
	discussion.ErrNoModelReason:    locales.ChatNeedModel,
	discussion.ErrInputRequired:    locales.ChatInputRequired,
	discussion.ErrNewChatRequired:  locales.ChatNewChatRequired,
	discussion.ErrAlreadyReplied:   locales.ChatAlreadyReplied,
	discussion.ErrRetryMissing:     locales.ChatRetryMissing,
	discussion.ErrEmptyReply:       locales.ChatEmptyReply,
	discussion.ErrSelectionInvalid: locales.ChatSelectionInvalid,
	discussion.ErrSelectionBlank:   locales.ChatSelectionBlank,
	discussion.ErrSourceMissing:    locales.ChatSourceMissing,
	discussion.ErrEmptyDigest:      locales.ChatEmptyDigest,
	model.ErrModelFields:           locales.SettingsModelFields,
}
