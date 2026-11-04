// Command extlocales checks extension catalogs and generates typed accessors.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/i18n"
)

const source = "zh-Hans"

var keyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
var types = map[string]string{"String": "string", "int": "int", "double": "float64", "num": "float64", "DateTime": "time.Time", "Object": "any"}

type entry struct {
	key string
	raw json.RawMessage
}
type parameter struct{ name, typ, ident string }
type message struct {
	key, text, description string
	params                 []parameter
}
type catalog struct {
	dir, id  string
	messages []message
}

// object preserves ARB property order, including placeholder parameter order.
func object(data []byte) ([]entry, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return nil, fmt.Errorf("expected JSON object")
	}
	var entries []entry
	seen := map[string]bool{}
	for d.More() {
		t, err = d.Token()
		if err != nil {
			return nil, err
		}
		key := t.(string)
		if seen[key] {
			return nil, fmt.Errorf("%s: duplicate property", key)
		}
		seen[key] = true
		var raw json.RawMessage
		if err = d.Decode(&raw); err != nil {
			return nil, err
		}
		entries = append(entries, entry{key, raw})
	}
	return entries, nil
}

func read(path string) ([]entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	entries, err := object(data)
	if err != nil {
		return nil, fmt.Errorf("%s: <file>: %w", path, err)
	}
	return entries, nil
}

func languageFiles(dir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, path := range paths {
		tags = append(tags, strings.TrimSuffix(filepath.Base(path), ".json"))
	}
	return tags, nil
}

func compatible(typ, kind string) bool {
	switch kind {
	case "number", "plural", "selectordinal":
		return typ == "int" || typ == "double" || typ == "num" || typ == "Object"
	case "date", "time":
		return typ == "DateTime" || typ == "Object"
	case "select":
		return typ == "String" || typ == "Object"
	}
	return true
}

func identifier(name string) string {
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		return "arg" + name
	}
	if token.Lookup(name).IsKeyword() || slices.Contains([]string{"msgs", "i18n", "In", "Catalog", "time", "string", "any", "int", "float64", "nil", "true", "false"}, name) {
		name += "_"
	}
	return name
}

func functionName(key string) string {
	var name strings.Builder
	for _, word := range strings.Split(key, "_") {
		if word != "" {
			name.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
	}
	return name.String()
}

func metadata(raw json.RawMessage) (string, []parameter, error) {
	if len(raw) == 0 {
		return "", nil, nil
	}
	var meta struct {
		Description  json.RawMessage `json:"description"`
		Placeholders json.RawMessage `json:"placeholders"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return "", nil, err
	}
	var description string
	if len(meta.Description) != 0 {
		if err := json.Unmarshal(meta.Description, &description); err != nil {
			return "", nil, err
		}
		if strings.TrimSpace(description) == "" {
			return "", nil, fmt.Errorf("description must not be blank")
		}
	}
	var params []parameter
	if len(meta.Placeholders) != 0 {
		entries, err := object(meta.Placeholders)
		if err != nil {
			return "", nil, err
		}
		names := map[string]bool{}
		for _, e := range entries {
			var p struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(e.raw, &p); err != nil {
				return "", nil, err
			}
			if types[p.Type] == "" {
				return "", nil, fmt.Errorf("placeholder %s: unsupported type %q", e.key, p.Type)
			}
			ident := identifier(e.key)
			if !token.IsIdentifier(ident) || names[ident] {
				return "", nil, fmt.Errorf("placeholder %s: invalid or colliding parameter name", e.key)
			}
			names[ident] = true
			params = append(params, parameter{e.key, p.Type, ident})
		}
	}
	return description, params, nil
}

func validateMessage(path string, m message, raw json.RawMessage) error {
	fail := func(f string, args ...any) error {
		return fmt.Errorf("%s: %s: %s", path, m.key, fmt.Sprintf(f, args...))
	}
	var text string
	if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &text) != nil {
		return fail("message must be a string")
	}
	parsed, err := i18n.ParseMessage(text)
	if err != nil {
		return fail("%v", err)
	}
	found := map[string]bool{}
	for _, arg := range parsed.Args() {
		idx := slices.IndexFunc(m.params, func(p parameter) bool { return p.name == arg.Name })
		if idx < 0 {
			return fail("undeclared argument %s", arg.Name)
		}
		if !compatible(m.params[idx].typ, arg.Kind) {
			return fail("argument %s: type %s cannot be %s", arg.Name, m.params[idx].typ, arg.Kind)
		}
		found[arg.Name] = true
	}
	for _, p := range m.params {
		if !found[p.name] {
			return fail("missing argument %s", p.name)
		}
	}
	return nil
}

func load(root string) ([]catalog, error) {
	hostDir := filepath.Join(root, "locales")
	tags, err := languageFiles(hostDir)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(tags, source) {
		return nil, fmt.Errorf("%s: <file>: missing source language", hostDir)
	}
	owners := map[string]string{}
	for _, tag := range tags {
		path := filepath.Join(hostDir, tag+".json")
		entries, err := read(path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !strings.HasPrefix(e.key, "@") {
				owners[e.key] = path
			}
		}
	}
	dirs, err := filepath.Glob(filepath.Join(root, "ext", "*", "*text"))
	if err != nil {
		return nil, err
	}
	var catalogs []catalog
	for _, dir := range dirs {
		id := filepath.Base(filepath.Dir(dir))
		if filepath.Base(dir) != id+"text" || !token.IsIdentifier(id+"text") || !keyPattern.MatchString(id) {
			return nil, fmt.Errorf("%s: <directory>: expected ext/%s/%stext", dir, id, id)
		}
		langs, err := languageFiles(dir)
		if err != nil {
			return nil, err
		}
		if !slices.Equal(tags, langs) {
			return nil, fmt.Errorf("%s: <languages>: got %v, want %v", dir, langs, tags)
		}
		path := filepath.Join(dir, source+".json")
		entries, err := read(path)
		if err != nil {
			return nil, err
		}
		metas := map[string]json.RawMessage{}
		for _, e := range entries {
			metas[e.key] = e.raw
		}
		cat := catalog{dir: dir, id: id}
		names := map[string]bool{"Catalog": true, "Messages": true, "In": true}
		for _, e := range entries {
			if strings.HasPrefix(e.key, "@") {
				continue
			}
			if !keyPattern.MatchString(e.key) {
				return nil, fmt.Errorf("%s: %s: invalid key", path, e.key)
			}
			name := functionName(e.key)
			if names[name] {
				return nil, fmt.Errorf("%s: %s: generated name collision", path, e.key)
			}
			names[name] = true
			if owner := owners[e.key]; owner != "" {
				return nil, fmt.Errorf("%s: %s: duplicate key also in %s", path, e.key, owner)
			}
			owners[e.key] = path
			desc, params, err := metadata(metas["@"+e.key])
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %w", path, e.key, err)
			}
			m := message{key: e.key, description: desc, params: params}
			if err := validateMessage(path, m, e.raw); err != nil {
				return nil, err
			}
			_ = json.Unmarshal(e.raw, &m.text)
			cat.messages = append(cat.messages, m)
		}
		for _, tag := range langs {
			path := filepath.Join(dir, tag+".json")
			entries, err := read(path)
			if err != nil {
				return nil, err
			}
			present := map[string]bool{}
			for _, e := range entries {
				if e.key == "@@locale" {
					var locale string
					_ = json.Unmarshal(e.raw, &locale)
					if locale != tag {
						return nil, fmt.Errorf("%s: @@locale: want %s", path, tag)
					}
					continue
				}
				if strings.HasPrefix(e.key, "@") {
					continue
				}
				idx := slices.IndexFunc(cat.messages, func(m message) bool { return m.key == e.key })
				if idx < 0 {
					return nil, fmt.Errorf("%s: %s: key not in source", path, e.key)
				}
				if err := validateMessage(path, cat.messages[idx], e.raw); err != nil {
					return nil, err
				}
				present[e.key] = true
			}
			for _, m := range cat.messages {
				if !present[m.key] {
					return nil, fmt.Errorf("%s: %s: missing key", path, m.key)
				}
			}
		}
		catalogs = append(catalogs, cat)
	}
	return catalogs, nil
}

func generate(cat catalog) ([]byte, error) {
	var b strings.Builder
	w := func(f string, args ...any) { fmt.Fprintf(&b, f, args...) }
	w("// Code generated by go run ./internal/extlocales. DO NOT EDIT.\n\npackage %stext\n\nimport (\n\"embed\"\n", cat.id)
	for _, m := range cat.messages {
		if slices.ContainsFunc(m.params, func(p parameter) bool { return p.typ == "DateTime" }) {
			w("\"time\"\n")
			break
		}
	}
	w("\n\"github.com/egoist/mygo/i18n\"\n)\n\n//go:embed *.json\nvar files embed.FS\n\n")
	w("// Catalog holds this extension's messages, with the %s namespace.\nvar Catalog *i18n.Catalog\n\n", cat.id)
	w("func init() {\nvar err error\nif Catalog, err = i18n.NewLibraryCatalog(files, %q, %q); err != nil { panic(err) }\n}\n\n", source, cat.id)
	w("// Messages say the messages in one language.\ntype Messages struct{ locale string }\n\n")
	w("// In returns the messages in the language of locale, as \"de\" or\n// \"zh-Hant\", or of the catalog's language that suits it best, \"\" for the\n// source language.\nfunc In(locale string) Messages { return Messages{locale} }\n")
	for _, m := range cat.messages {
		name := functionName(m.key)
		var params, names, values []string
		for _, p := range m.params {
			params = append(params, p.ident+" "+types[p.typ])
			names = append(names, p.ident)
			values = append(values, strconv.Quote(p.name)+": "+p.ident)
		}
		args := "nil"
		if len(values) > 0 {
			args = "map[string]any{" + strings.Join(values, ", ") + "}"
		}
		doc := func(where string) {
			w("\n// %s says %s\n// in %s.\n", name, strconv.Quote(m.text), where)
			if m.description != "" {
				w("//\n// %s\n", strings.Join(strings.Fields(m.description), " "))
			}
		}
		doc("the language the app shows")
		w("func %s(%s) string { return In(i18n.Locale()).%s(%s) }\n", name, strings.Join(params, ", "), name, strings.Join(names, ", "))
		doc("the language of msgs")
		w("func (msgs Messages) %s(%s) string { return Catalog.Format(msgs.locale, %q, %s) }\n", name, strings.Join(params, ", "), m.key, args)
	}
	return format.Source([]byte(b.String()))
}

func run(root string, check bool) error {
	catalogs, err := load(root)
	if err != nil {
		return err
	}
	outputs := map[string][]byte{}
	for _, cat := range catalogs {
		data, err := generate(cat)
		if err != nil {
			return fmt.Errorf("%s: <generated>: %w", cat.dir, err)
		}
		path := filepath.Join(cat.dir, "catalog_gen.go")
		if check {
			old, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(old, data) {
				return fmt.Errorf("%s: <generated>: stale; run npm run ext-locales", path)
			}
		}
		outputs[path] = data
	}
	if !check {
		for path, data := range outputs {
			if err := os.WriteFile(path, data, 0644); err != nil {
				return err
			}
		}
	}
	fmt.Printf("ext i18n ok: %d catalogs checked; catalog_gen.go is current\n", len(catalogs))
	return nil
}

func main() {
	check := flag.Bool("check", false, "check without writing generated files")
	flag.Parse()
	if err := run(".", *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
