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
			{model.ErrRequestTooLarge, locales.StatusRequestTooLarge()},
			{model.ErrTruncated, locales.StatusTruncated()},
			{model.ErrKeyNewline, locales.SettingsKeyNewline()},
			{model.ErrRedirect, locales.StatusRedirect()},
			{model.ErrModelRequired, locales.SettingsModelRequired()},
			{model.ErrNegativeTokens, locales.StatusNegativeTokens()},
			{model.ErrMessagesRequired, locales.StatusMessagesRequired()},
			{model.ErrToolJSON, locales.StatusToolJSON()},
			{model.ErrDisconnected, locales.StatusDisconnected()},
			{model.ErrFiltered, locales.StatusFiltered()},
			{model.ErrIncomplete, locales.StatusIncomplete()},
			{model.ErrReplyTooLarge, locales.StatusReplyTooLarge()},
			{model.ErrNoText, locales.StatusNoText()},
			{model.ErrToolCallMissing, locales.StatusToolCallMissing()},
			{model.ErrRole, locales.StatusRole()},
			{model.ErrService, locales.StatusService()},
			{model.ErrConnection, locales.StatusConnection()},
			{model.ErrDestination, locales.StatusDestination()},
			{model.ErrConfigTooLarge, locales.SettingsConfigTooLarge()},
			{model.ErrConfigYAML, locales.SettingsConfigYAML()},
			{model.ErrConfigDocuments, locales.SettingsConfigDocuments()},
			{model.ErrConfigEncode, locales.SettingsConfigEncode()},
			{model.ErrIDFormat, locales.SettingsIDRule()},
			{model.ErrConfigVersion, locales.SettingsConfigVersion()},
			{model.ErrProviderDuplicate, locales.SettingsProviderDuplicate()},
			{model.ErrNameRequired, locales.SettingsNameRequired()},
			{model.ErrProtocol, locales.SettingsProtocolInvalid()},
			{model.ErrEnvFormat, locales.SettingsEnvRule()},
			{model.ErrURLScheme, locales.SettingsURLScheme()},
			{model.ErrURLHost, locales.SettingsURLNoHost()},
			{model.ErrURLExtra, locales.SettingsURLExtra()},
			{model.ErrModelID, locales.SettingsModelIDInvalid()},
			{model.ErrCatalogTotal, locales.SettingsCatalogTotal()},
			{model.ErrCatalogJSON, locales.SettingsCatalogJSON()},
			{model.ErrCatalogService, locales.SettingsCatalogService()},
			{model.ErrCatalogData, locales.SettingsCatalogData()},
			{model.ErrCatalogPagination, locales.SettingsCatalogPagination()},
			{model.ErrCatalogPages, locales.SettingsCatalogPages()},
			{model.ErrRequestCreate, locales.StatusRequestCreate()},
			{model.ErrCatalogPageSize, locales.SettingsCatalogPageSize()},
			{model.ErrKeyDelete, locales.SettingsKeyDelete()},
			{model.ErrKeySave, locales.SettingsKeySave()},
			{model.ErrKeychain, locales.SettingsKeychain()},
			{ai.ErrOff, locales.ChatOff()},
			{ai.ErrChatModel, locales.ChatNeedModel()},
			{inference.ErrTrailingJSON, locales.StatusTrailingJSON()},
			{writing.ErrNoModelReason, locales.EditorWhyPickModel()},
			{writing.ErrChapterMissing, locales.StatusChapterMissing()},
			{writing.ErrNoSetting, locales.CheckNoSetting()},
			{discussion.ErrChapterGone, locales.ChatChapterGone()},
			{discussion.ErrNoModelReason, locales.ChatNeedModel()},
			{discussion.ErrInputRequired, locales.ChatInputRequired()},
			{discussion.ErrNewChatRequired, locales.ChatNewChatRequired()},
			{discussion.ErrAlreadyReplied, locales.ChatAlreadyReplied()},
			{discussion.ErrRetryMissing, locales.ChatRetryMissing()},
			{discussion.ErrEmptyReply, locales.ChatEmptyReply()},
			{discussion.ErrSelectionInvalid, locales.ChatSelectionInvalid()},
			{discussion.ErrSelectionBlank, locales.ChatSelectionBlank()},
			{discussion.ErrSourceMissing, locales.ChatSourceMissing()},
			{discussion.ErrEmptyDigest, locales.ChatEmptyDigest()},
			{&model.LimitsError{ID: "x", Rule: "integer"}, locales.SettingsModelAtInt("x")},
			{model.ErrModelFields, locales.SettingsModelFields()},
			{&model.LimitsError{ID: "x", Rule: "duplicate"}, locales.SettingsModelDup("x")},
			{&model.LimitsError{ID: "x", Rule: "numbers"}, locales.SettingsModelNumbers("x")},
			{&model.LimitsError{ID: "x", Rule: "output"}, locales.SettingsModelOutMax("x")},
			{&model.LimitsError{ID: "x", Rule: "threshold"}, locales.SettingsModelAtRange("x")},
			{&model.ContextLimitError{Part: "outline"}, locales.StatusContextOutline()},
			{&model.ContextLimitError{Part: "chapter"}, locales.StatusContextChapter()},
			{&model.ContextLimitError{Part: "lore"}, locales.StatusContextLore()},
			{&model.ContextLimitError{Part: "turn"}, locales.StatusContextTurn()},
			{&model.ContextLimitError{Part: "message"}, locales.StatusContextMessage()},
			{&model.ContextLimitError{Part: "index"}, locales.StatusContextIndex()},
			{&model.ContextLimitError{Part: "summary"}, locales.StatusContextSummary()},
			{&model.ContextLimitError{Part: "attachments"}, locales.StatusContextAttachments()},
			{&model.ConfigError{Op: "locate", Err: errors.New("detail")}, locales.SettingsConfigLocate("detail")},
			{&model.ConfigError{Op: "open", Err: errors.New("detail")}, locales.SettingsConfigOpen("detail")},
			{&model.ConfigError{Op: "read", Err: errors.New("detail")}, locales.SettingsConfigRead("detail")},
			{&model.ConfigError{Op: "mkdir", Err: errors.New("detail")}, locales.SettingsConfigMkdir("detail")},
			{&model.ConfigError{Op: "create", Err: errors.New("detail")}, locales.SettingsConfigCreate("detail")},
			{&model.ConfigError{Op: "replace", Err: errors.New("detail")}, locales.SettingsConfigReplace("detail")},
			{&model.ConfigError{Op: "write", Err: errors.New("detail")}, locales.SettingsConfigWrite("detail")},
			{&model.ProviderError{Count: 2, Err: errors.New("detail")}, locales.SettingsProviderError(2, "detail")},
			{&model.ProviderMissingError{ID: "x"}, locales.SettingsProviderMissing("x")},
			{&model.EffortError{Effort: "x"}, locales.SettingsEffortError("x")},
			{&model.ToolError{Name: "x"}, locales.ChatToolError("x")},
			{&model.TimeoutError{Err: errors.New("detail")}, locales.StatusModelTimeout("detail")},
			{&model.AbortedError{Err: errors.New("detail")}, locales.StatusModelAborted("detail")},
			{&inference.JSONError{Err: errors.New("detail")}, locales.StatusJSONError("detail")},
			{&inference.OutputLimitError{Limit: 2}, locales.StatusOutputLimit("2")},
			{&writing.ShortChapterError{Count: 2}, locales.CheckShortChapter(2)},
			{&writing.CheckResultError{Err: errors.New("detail")}, locales.CheckResultError("detail")},
			{&discussion.CompactError{Err: errors.New("detail")}, locales.ChatCompactError("detail")},
			{&discussion.ReplyError{Err: errors.New("detail")}, locales.ChatReplyError("detail")},
			{&model.HTTPError{Code: 401}, locales.StatusHTTPAuth(401)},
			{&model.HTTPError{Code: 404}, locales.StatusHTTPMissing(404)},
			{&model.HTTPError{Code: 429}, locales.StatusHTTPRate(429)},
			{&model.HTTPError{Code: 400}, locales.StatusHTTPRejected(400)},
			{&model.HTTPError{Code: 500}, locales.StatusHTTPServer(500)},
			{&model.HTTPError{Code: 418}, locales.StatusHTTPOther(418)},
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
	if got, want := text(outer), locales.ChatReplyError(locales.StatusContextMessage()); got != want {
		t.Fatalf("%q != %q", got, want)
	}
	config := &model.ConfigError{Op: "read", Err: os.ErrPermission}
	if got, want := text(config), locales.SettingsConfigRead(os.ErrPermission.Error()); got != want {
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
