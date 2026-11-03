package desk

import (
	"testing"

	"github.com/egoist/mygo/i18n"
	"writer/workspace"
)

func TestChatTablesFollowLocale(t *testing.T) {
	tag := "en"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	for _, tc := range []struct{ tag, part, fate, where, feature string }{
		{"en", "Chapter Cards", "Accepted", "After a Sentence", "Chat"},
		{"zh-Hans", "本章的卡片", "已采纳", "接在一句之后", "讨论"},
	} {
		tag = tc.tag
		for label, pair := range map[string][2]string{
			"part":    {partNames[workspace.PartCards](), tc.part},
			"fate":    {fateNames[workspace.FateTaken](), tc.fate},
			"where":   {whereNames[workspace.ProposeAfter](), tc.where},
			"feature": {featureName(workspace.FeatureTalk), tc.feature},
		} {
			if pair[0] != pair[1] {
				t.Errorf("%s %s: got %q, want %q", tag, label, pair[0], pair[1])
			}
		}
	}
}
