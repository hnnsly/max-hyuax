//go:build integration

package httpapi_test

import (
	"encoding/json/v2"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// dataAPI — формат DATA-API.yaml в корне репозитория (обязательные проверки для жюри).
type dataAPI struct {
	BaseURL string `yaml:"base_url"`
	Checks  []struct {
		ID     string `yaml:"id"`
		Method string `yaml:"method"`
		Path   string `yaml:"path"`
		Role   string `yaml:"role"`
		Params struct {
			Path    map[string]string `yaml:"path"`
			Query   map[string]string `yaml:"query"`
			Headers map[string]string `yaml:"headers"`
			Body    map[string]any    `yaml:"body"`
		} `yaml:"params"`
		Expect struct {
			Status      []int    `yaml:"status"`
			ContentType string   `yaml:"content_type"`
			Body        string   `yaml:"body"`
			Required    []string `yaml:"required_fields"`
			Required200 []string `yaml:"required_fields_on_200"`
			Required409 []string `yaml:"required_fields_on_409"`
		} `yaml:"expect"`
	} `yaml:"checks"`
}

func readYAML(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// Каждая проверка из DATA-API.yaml проходит на настоящем API и описана в openapi.yaml.
func TestDataAPIChecksPass(t *testing.T) {
	var spec dataAPI
	readYAML(t, "../../../../DATA-API.yaml", &spec)
	var contract struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	readYAML(t, "../../../../api/openapi.yaml", &contract)
	if !strings.HasSuffix(spec.BaseURL, "/api/v1") || len(spec.Checks) < 6 {
		t.Fatalf("base_url = %q, checks = %d", spec.BaseURL, len(spec.Checks))
	}

	tokens := map[string]string{}
	for _, c := range spec.Checks {
		t.Run(c.ID, func(t *testing.T) {
			if _, ok := contract.Paths[c.Path][strings.ToLower(c.Method)]; !ok {
				t.Fatalf("%s %s is not in openapi.yaml", c.Method, c.Path)
			}
			path := c.Path
			for k, v := range c.Params.Path {
				path = strings.ReplaceAll(path, "{"+k+"}", url.PathEscape(v))
			}
			if len(c.Params.Query) > 0 {
				q := url.Values{}
				for k, v := range c.Params.Query {
					q.Set(k, v)
				}
				path += "?" + q.Encode()
			}
			var body []byte
			if c.Params.Body != nil {
				b, err := json.Marshal(c.Params.Body)
				if err != nil {
					t.Fatal(err)
				}
				body = b
			}
			token := ""
			if c.Role != "none" {
				if tokens[c.Role] == "" {
					tokens[c.Role] = login(t, c.Role)
				}
				token = tokens[c.Role]
			}

			r := callRaw(t, api, c.Method, "/api/v1"+path, token, body)
			if !slices.Contains(c.Expect.Status, r.status) {
				t.Fatalf("status %d, want one of %v; body %v", r.status, c.Expect.Status, r.body)
			}
			if !strings.HasPrefix(r.ctype, c.Expect.ContentType) {
				t.Fatalf("content type %q, want %q", r.ctype, c.Expect.ContentType)
			}
			fields := c.Expect.Required
			switch r.status {
			case 200:
				fields = append(fields, c.Expect.Required200...)
			case 409:
				fields = append(fields, c.Expect.Required409...)
			}
			var doc any = r.body
			if c.Expect.Body == "array" {
				doc = r.list
			}
			for _, f := range fields {
				if !hasField(doc, f) {
					t.Errorf("response has no %q: %v%v", f, r.body, r.list)
				}
			}
		})
	}
}

// hasField проверяет путь «a.b» в объекте или «[].a.b» у каждого элемента непустого массива.
func hasField(doc any, path string) bool {
	if rest, ok := strings.CutPrefix(path, "[]."); ok {
		list, _ := doc.([]any)
		if len(list) == 0 {
			return false
		}
		for _, item := range list {
			if !hasField(item, rest) {
				return false
			}
		}
		return true
	}
	head, rest, nested := strings.Cut(path, ".")
	obj, _ := doc.(map[string]any)
	v, ok := obj[head]
	if !ok {
		return false
	}
	return !nested || hasField(v, rest)
}

func TestHasField(t *testing.T) {
	doc := map[string]any{"user": map[string]any{"id": 1.0}, "none": nil}
	list := []any{map[string]any{"id": "a"}, map[string]any{"name": "b"}}
	for _, c := range []struct {
		doc  any
		path string
		want bool
	}{
		{doc, "user.id", true},
		{doc, "none", true}, // null допустим: поле есть
		{doc, "user.name", false},
		{doc, "token", false},
		{list, "[].id", false}, // у второго элемента нет id
		{[]any{}, "[].id", false},
		{list[:1], "[].id", true},
	} {
		if got := hasField(c.doc, c.path); got != c.want {
			t.Errorf("hasField(%v, %q) = %v, want %v", c.doc, c.path, got, c.want)
		}
	}
}
