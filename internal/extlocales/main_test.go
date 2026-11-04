package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"writer/internal/testtmp"

	"github.com/egoist/mygo/i18n"
)

const validSource = `{"hello":"你好 {name}","@hello":{"description":"A greeting.","placeholders":{"name":{"type":"String"}}}}`
const validEnglish = `{"hello":"Hello {name}"}`

func TestMain(m *testing.M) { testtmp.Main(m) }

func put(t *testing.T, root, path, text string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	put(t, root, "locales/zh-Hans.json", `{"host":"宿主"}`)
	put(t, root, "locales/en.json", `{"host":"Host"}`)
	put(t, root, "ext/demo/demotext/zh-Hans.json", validSource)
	put(t, root, "ext/demo/demotext/en.json", validEnglish)
	return root
}

func reject(t *testing.T, path, text, want string) {
	t.Helper()
	root := fixture(t)
	put(t, root, path, text)
	err := run(root, false)
	if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), filepath.Base(path)) {
		t.Fatalf("got %v; want %s and filename", err, want)
	}
	if _, err := os.Stat(filepath.Join(root, "ext/demo/demotext/catalog_gen.go")); !os.IsNotExist(err) {
		t.Fatal("invalid catalog wrote generated code")
	}
}

func TestRejectInvalidJSON(t *testing.T) { reject(t, "ext/demo/demotext/en.json", `{`, "invalid JSON") }

func TestRejectInvalidKey(t *testing.T) {
	reject(t, "ext/demo/demotext/zh-Hans.json", `{"bad.key":"x","@bad.key":{"description":"x"}}`, "invalid key")
}

func TestRejectGeneratedFunctionNameCollision(t *testing.T) {
	reject(t, "ext/demo/demotext/zh-Hans.json", `{"a_b":"x","@a_b":{"description":"x"},"aB":"y","@aB":{"description":"y"}}`, "generated name collision")
}

func TestGeneratedNamesFollowMygo(t *testing.T) {
	if functionName("some_key") != "SomeKey" {
		t.Fatal("underscore word capitalization")
	}
	for name, want := range map[string]string{"type": "type_", "time": "time_", "0": "arg0", "name": "name"} {
		if got := identifier(name); got != want {
			t.Fatalf("%s: got %s, want %s", name, got, want)
		}
	}
}

func TestRejectMissingMetadata(t *testing.T) {
	reject(t, "ext/demo/demotext/zh-Hans.json", `{"hello":"你好 {name}"}`, "undeclared argument name")
}

func TestRejectBlankDescription(t *testing.T) {
	reject(t, "ext/demo/demotext/zh-Hans.json", `{"hello":"你好","@hello":{"description":" \t\n"}}`, "description must not be blank")
}

func TestOptionalMetadataAndDescription(t *testing.T) {
	for _, sourceText := range []string{
		`{"hello":"你好"}`,
		`{"hello":"你好","@hello":{}}`,
		`{"hello":"你好 {name}","@hello":{"placeholders":{"name":{"type":"String"}}}}`,
	} {
		t.Run(sourceText, func(t *testing.T) {
			root := fixture(t)
			put(t, root, "ext/demo/demotext/zh-Hans.json", sourceText)
			english := `{"hello":"Hello"}`
			if strings.Contains(sourceText, "placeholders") {
				english = validEnglish
			}
			put(t, root, "ext/demo/demotext/en.json", english)
			if err := run(root, false); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(root, "ext/demo/demotext/catalog_gen.go"))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte("//\n")) {
				t.Fatal("accessors without descriptions have a description paragraph")
			}
			if err := run(root, true); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRejectExtraTranslationKey(t *testing.T) {
	reject(t, "ext/demo/demotext/en.json", `{"hello":"Hello {name}","extra":"x"}`, "extra: key not in source")
}

func TestRejectMissingTranslationKey(t *testing.T) {
	reject(t, "ext/demo/demotext/en.json", `{}`, "hello: missing key")
}

func TestRejectExtraArgument(t *testing.T) {
	reject(t, "ext/demo/demotext/en.json", `{"hello":"{name} {other}"}`, "undeclared argument other")
}

func TestRejectMissingArgument(t *testing.T) {
	reject(t, "ext/demo/demotext/en.json", `{"hello":"Hello"}`, "missing argument name")
}

func TestRejectIncompatibleArgumentType(t *testing.T) {
	reject(t, "ext/demo/demotext/en.json", `{"hello":"{name, number}"}`, "type String cannot be number")
}

func TestRejectUnsupportedPlaceholderType(t *testing.T) {
	reject(t, "ext/demo/demotext/zh-Hans.json", strings.Replace(validSource, "String", "float", 1), "unsupported type")
}

func TestRejectUnparseableMessage(t *testing.T) {
	reject(t, "ext/demo/demotext/en.json", `{"hello":"{name"}`, "hello:")
}

func TestRejectHostKeyCollision(t *testing.T) {
	reject(t, "locales/en.json", `{"hello":"Host"}`, "duplicate key")
}

func TestRejectExtensionKeyCollision(t *testing.T) {
	root := fixture(t)
	put(t, root, "ext/other/othertext/zh-Hans.json", validSource)
	put(t, root, "ext/other/othertext/en.json", validEnglish)
	if _, err := load(root); err == nil || !strings.Contains(err.Error(), "hello: duplicate key") {
		t.Fatal(err)
	}
}

func TestRejectWrongDirectoryName(t *testing.T) {
	root := fixture(t)
	put(t, root, "ext/other/wrongtext/zh-Hans.json", validSource)
	if _, err := load(root); err == nil || !strings.Contains(err.Error(), "expected ext/other/othertext") {
		t.Fatal(err)
	}
}

func TestRejectDifferentLanguageSet(t *testing.T) {
	root := fixture(t)
	put(t, root, "ext/demo/demotext/fr.json", validEnglish)
	if _, err := load(root); err == nil || !strings.Contains(err.Error(), "<languages>") {
		t.Fatal(err)
	}
}

func TestRejectStaleGeneratedCodeWithoutWriting(t *testing.T) {
	root := fixture(t)
	path := filepath.Join(root, "ext/demo/demotext/catalog_gen.go")
	put(t, root, "ext/demo/demotext/catalog_gen.go", "stale\n")
	if err := run(root, true); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "stale\n" {
		t.Fatal("check wrote file", err)
	}
}

func TestGeneratedAccessorsParseAndKeepParameterOrder(t *testing.T) {
	root := fixture(t)
	text := `{"all":"{count, number} {folder} {price, number} {ratio, number} {day, date} {object}","@all":{"description":"All types.","placeholders":{"count":{"type":"int"},"folder":{"type":"String"},"price":{"type":"double"},"ratio":{"type":"num"},"day":{"type":"DateTime"},"object":{"type":"Object"}}}}`
	put(t, root, "ext/demo/demotext/zh-Hans.json", text)
	put(t, root, "ext/demo/demotext/en.json", text)
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "ext/demo/demotext/catalog_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "catalog_gen.go", data, parser.AllErrors); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("All(count int, folder string, price float64, ratio float64, day time.Time, object any)")) {
		t.Fatal(string(data))
	}
	if bytes.Contains(data, []byte("i18n.Register")) || bytes.Contains(data, []byte("mygo/locales/")) || bytes.Contains(data, []byte("\r")) || !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatal("invalid library output")
	}
	if err := run(root, true); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryMissingTranslationFallsBackToChinese(t *testing.T) {
	root := fixture(t)
	put(t, root, "ext/demo/demotext/en.json", `{}`)
	dir := filepath.Join(root, "ext/demo/demotext")
	lib, err := i18n.NewLibraryCatalog(os.DirFS(dir), source, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got := lib.Format("en", "hello", map[string]any{"name": "Ada"}); got != "你好 Ada" {
		t.Fatal(got)
	}
	if _, err := load(root); err == nil || !strings.Contains(err.Error(), "hello: missing key") {
		t.Fatal("strict checker accepted missing translation", err)
	}
}
