package aitext

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/egoist/mygo/i18n"
	"writer/ai"
	"writer/discussion"
	"writer/inference"
	"writer/locales"
	"writer/model"
	"writer/writing"
)

func TestTypedAIErrorsTranslateInBothLocales(t *testing.T) {
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "zh-Hans" }) })
	for _, language := range []string{"zh-Hans", "en"} {
		tag = language
		for _, tc := range []struct {
			err  error
			want string
		}{
			{model.ErrRequestTooLarge, StatusRequestTooLarge()},
			{model.ErrTruncated, StatusTruncated()},
			{model.ErrKeyNewline, SettingsKeyNewline()},
			{model.ErrRedirect, StatusRedirect()},
			{model.ErrModelRequired, SettingsModelRequired()},
			{model.ErrNegativeTokens, StatusNegativeTokens()},
			{model.ErrMessagesRequired, StatusMessagesRequired()},
			{model.ErrToolJSON, StatusToolJSON()},
			{model.ErrDisconnected, StatusDisconnected()},
			{model.ErrFiltered, StatusFiltered()},
			{model.ErrIncomplete, StatusIncomplete()},
			{model.ErrReplyTooLarge, StatusReplyTooLarge()},
			{model.ErrNoText, StatusNoText()},
			{model.ErrToolCallMissing, StatusToolCallMissing()},
			{model.ErrRole, StatusRole()},
			{model.ErrService, StatusService()},
			{model.ErrConnection, StatusConnection()},
			{model.ErrDestination, StatusDestination()},
			{model.ErrConfigTooLarge, SettingsConfigTooLarge()},
			{model.ErrConfigYAML, SettingsConfigYAML()},
			{model.ErrConfigDocuments, SettingsConfigDocuments()},
			{model.ErrConfigEncode, SettingsConfigEncode()},
			{model.ErrIDFormat, SettingsIDRule()},
			{model.ErrConfigVersion, SettingsConfigVersion()},
			{model.ErrProviderDuplicate, SettingsProviderDuplicate()},
			{model.ErrNameRequired, SettingsNameRequired()},
			{model.ErrProtocol, SettingsProtocolInvalid()},
			{model.ErrEnvFormat, SettingsEnvRule()},
			{model.ErrURLScheme, SettingsURLScheme()},
			{model.ErrURLHost, SettingsURLNoHost()},
			{model.ErrURLExtra, SettingsURLExtra()},
			{model.ErrModelID, SettingsModelIDInvalid()},
			{model.ErrCatalogTotal, SettingsCatalogTotal()},
			{model.ErrCatalogJSON, SettingsCatalogJSON()},
			{model.ErrCatalogService, SettingsCatalogService()},
			{model.ErrCatalogData, SettingsCatalogData()},
			{model.ErrCatalogPagination, SettingsCatalogPagination()},
			{model.ErrCatalogPages, SettingsCatalogPages()},
			{model.ErrRequestCreate, StatusRequestCreate()},
			{model.ErrCatalogPageSize, SettingsCatalogPageSize()},
			{model.ErrKeyDelete, SettingsKeyDelete()},
			{model.ErrKeySave, SettingsKeySave()},
			{model.ErrKeychain, SettingsKeychain()},
			{ai.ErrOff, ChatOff()},
			{ai.ErrChatModel, locales.ChatNeedModel()},
			{inference.ErrTrailingJSON, StatusTrailingJSON()},
			{writing.ErrNoModelReason, EditorWhyPickModel()},
			{writing.ErrChapterMissing, StatusChapterMissing()},
			{writing.ErrNoSetting, CheckNoSetting()},
			{discussion.ErrChapterGone, locales.ChatChapterGone()},
			{discussion.ErrNoModelReason, locales.ChatNeedModel()},
			{discussion.ErrInputRequired, ChatInputRequired()},
			{discussion.ErrNewChatRequired, ChatNewChatRequired()},
			{discussion.ErrAlreadyReplied, ChatAlreadyReplied()},
			{discussion.ErrRetryMissing, ChatRetryMissing()},
			{discussion.ErrEmptyReply, ChatEmptyReply()},
			{discussion.ErrSelectionInvalid, ChatSelectionInvalid()},
			{discussion.ErrSelectionBlank, ChatSelectionBlank()},
			{discussion.ErrSourceMissing, ChatSourceMissing()},
			{discussion.ErrEmptyDigest, ChatEmptyDigest()},
			{&model.LimitsError{ID: "x", Rule: "integer"}, SettingsModelAtInt("x")},
			{model.ErrModelFields, SettingsModelFields()},
			{&model.LimitsError{ID: "x", Rule: "duplicate"}, SettingsModelDup("x")},
			{&model.LimitsError{ID: "x", Rule: "numbers"}, SettingsModelNumbers("x")},
			{&model.LimitsError{ID: "x", Rule: "output"}, SettingsModelOutMax("x")},
			{&model.LimitsError{ID: "x", Rule: "threshold"}, SettingsModelAtRange("x")},
			{&model.ContextLimitError{Part: "outline"}, StatusContextOutline()},
			{&model.ContextLimitError{Part: "chapter"}, StatusContextChapter()},
			{&model.ContextLimitError{Part: "lore"}, StatusContextLore()},
			{&model.ContextLimitError{Part: "turn"}, StatusContextTurn()},
			{&model.ContextLimitError{Part: "message"}, StatusContextMessage()},
			{&model.ContextLimitError{Part: "index"}, StatusContextIndex()},
			{&model.ContextLimitError{Part: "summary"}, StatusContextSummary()},
			{&model.ContextLimitError{Part: "attachments"}, StatusContextAttachments()},
			{&model.ConfigError{Op: "locate", Err: errors.New("detail")}, SettingsConfigLocate("detail")},
			{&model.ConfigError{Op: "open", Err: errors.New("detail")}, SettingsConfigOpen("detail")},
			{&model.ConfigError{Op: "read", Err: errors.New("detail")}, SettingsConfigRead("detail")},
			{&model.ConfigError{Op: "mkdir", Err: errors.New("detail")}, SettingsConfigMkdir("detail")},
			{&model.ConfigError{Op: "create", Err: errors.New("detail")}, SettingsConfigCreate("detail")},
			{&model.ConfigError{Op: "replace", Err: errors.New("detail")}, SettingsConfigReplace("detail")},
			{&model.ConfigError{Op: "write", Err: errors.New("detail")}, SettingsConfigWrite("detail")},
			{&model.ProviderError{Count: 2, Err: errors.New("detail")}, SettingsProviderError(2, "detail")},
			{&model.ProviderMissingError{ID: "x"}, SettingsProviderMissing("x")},
			{&model.EffortError{Effort: "x"}, SettingsEffortError("x")},
			{&model.ToolError{Name: "x"}, ChatToolError("x")},
			{&model.TimeoutError{Err: errors.New("detail")}, StatusModelTimeout("detail")},
			{&model.AbortedError{Err: errors.New("detail")}, StatusModelAborted("detail")},
			{&inference.JSONError{Err: errors.New("detail")}, StatusJSONError("detail")},
			{&inference.OutputLimitError{Limit: 2}, StatusOutputLimit("2")},
			{&writing.ShortChapterError{Count: 2}, CheckShortChapter(2)},
			{&writing.CheckResultError{Err: errors.New("detail")}, CheckResultError("detail")},
			{&discussion.CompactError{Err: errors.New("detail")}, ChatCompactError("detail")},
			{&discussion.ReplyError{Err: errors.New("detail")}, ChatReplyError("detail")},
			{&model.HTTPError{Code: 401}, StatusHTTPAuth(401)},
			{&model.HTTPError{Code: 404}, StatusHTTPMissing(404)},
			{&model.HTTPError{Code: 429}, StatusHTTPRate(429)},
			{&model.HTTPError{Code: 400}, StatusHTTPRejected(400)},
			{&model.HTTPError{Code: 500}, StatusHTTPServer(500)},
			{&model.HTTPError{Code: 418}, StatusHTTPOther(418)},
		} {
			got := text(tc.err)
			if got != tc.want || got == tc.err.Error() || tag == "en" && hasHan(got) {
				t.Errorf("%s %T: %q, want %q; developer %q", tag, tc.err, got, tc.want, tc.err.Error())
			}
		}
	}
}

func TestWrappedAIErrorsTranslateTheirCauses(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "zh-Hans" }) })
	inner := &model.ContextLimitError{Part: "message"}
	outer := &discussion.ReplyError{Err: inner}
	if got, want := text(outer), ChatReplyError(StatusContextMessage()); got != want {
		t.Fatalf("%q != %q", got, want)
	}
	config := &model.ConfigError{Op: "read", Err: os.ErrPermission}
	if got, want := text(config), SettingsConfigRead(os.ErrPermission.Error()); got != want {
		t.Fatalf("%q != %q", got, want)
	}
	if !errors.Is(outer, inner) || !errors.Is(config, os.ErrPermission) {
		t.Fatal("lost cause")
	}
}

func TestUnknownErrorsKeepOriginalText(t *testing.T) {
	for _, err := range []error{errors.New("unknown failure"), os.ErrPermission, sliceError{"unknown"}, fmt.Errorf("outer: %w", model.ErrKeyNewline)} {
		if _, ok := Text(err); ok {
			t.Fatalf("recognized unknown outer error %T", err)
		}
		if text(err) != err.Error() {
			t.Fatal(err)
		}
	}
}

type sliceError []string

func (e sliceError) Error() string { return "unknown slice error" }

func TestTypedAIErrorsUseAuthorCopy(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "zh-Hans" }) })
	for _, tc := range []struct {
		err  error
		want string
	}{
		{model.ErrProtocol, "协议必须是 chat、responses 或 anthropic"},
		{model.ErrModelID, "模型 ID 不能为空，前后不能有空格"},
		{ai.ErrChatModel, "请先配置讨论模型"},
		{model.ErrURLScheme, "地址要以 http:// 或 https:// 开头"},
		{&model.ProviderMissingError{ID: "x"}, "本机没有配置提供商 \"x\"，请在设置中「AI」配置里重新选择"},
		{&writing.ShortChapterError{Count: 2}, "这一章还不到 2 字，不用检查"},
	} {
		if got := text(tc.err); got != tc.want {
			t.Errorf("%T: %q, want %q", tc.err, got, tc.want)
		}
	}
}

func hasHan(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return unicode.Is(unicode.Han, r) })
}
