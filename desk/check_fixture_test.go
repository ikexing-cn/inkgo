package desk

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
	"writer/workspace"
)

const checkProse = "整条街的灯是同时灭的。\n林婉站在巷口，手电在工具箱最底下。她没有去摸，一个人走进了没有灯的巷子。\n老周在巷子那头等她，手里拎着一只旧灯泡。\n"

const checkAnswer = `{"findings":[
	{"kind":"conflict","title":"林晚怕黑，这里却一个人进了黑巷子","quote":"一个人走进了没有灯的巷子","entry":1,"says":"但她很怕黑"},
	{"kind":"name","title":"「林婉」是不是「林晚」写错了？","quote":"林婉","name":"林晚"},
	{"kind":"new","title":"老周","quote":"老周在巷子那头等她","name":"老周","category":"人物","text":"在巷子那头等林晚的人"}]}`

// checkFixture is a chapter at odds with the setting in three ways, and a
// model that says so.
func checkFixture(t *testing.T) (*App, *ui.Tester, string, *stream) {
	t.Helper()
	a, tt, id := writingFixture(t, checkProse+strings.Repeat("她蹲下去拧开灯座。", 30))
	a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角靠维修路灯谋生，但她很怕黑。她宁可绕路，沿着亮着的灯走回家。", Names: []string{"林晚"}})
	f := &stream{pieces: []string{checkAnswer}}
	penWith(a, f)
	tt.Frame()
	return a, tt, id, f
}
