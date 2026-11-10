package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	defaultOllamaURL   = "http://localhost:11434"
	defaultOllamaModel = "qwen3:0.6b"
)

type genLesson struct {
	Chars []string `json:"chars"`
	Text  string   `json:"text"`
	Count int      `json:"count"`
	Focus string   `json:"focus"`
}

type ollamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Format  string                 `json:"format,omitempty"`
	Think   *bool                  `json:"think,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

type ollamaTagsResponse struct {
	Models []struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	} `json:"models"`
}

// normalizeOllamaURL accepts either a URL or a bare host:port (as used by
// OLLAMA_HOST) and returns a canonical base URL.
func normalizeOllamaURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		s = defaultOllamaURL
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	return strings.TrimRight(s, "/")
}

func listOllamaModels(baseURL string) ([]string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(normalizeOllamaURL(baseURL) + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("cannot reach Ollama at %s (%v)\nStart it with 'ollama serve' or pass -ollama URL", baseURL, err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama at %s returned HTTP %d: %s", baseURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tags ollamaTagsResponse
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("invalid model list from Ollama at %s: %v", baseURL, err)
	}

	var models []string
	seen := map[string]bool{}
	for _, m := range tags.Models {
		name := m.Name
		if name == "" {
			name = m.Model
		}
		if name != "" && !seen[name] {
			seen[name] = true
			models = append(models, name)
		}
	}
	sort.Strings(models)
	return models, nil
}

func matchOllamaModel(models []string, want string) string {
	want = strings.TrimSpace(want)
	if want == "" {
		return ""
	}
	for _, m := range models {
		if strings.EqualFold(m, want) {
			return m
		}
	}

	// An explicit tag must match exactly. Only an untagged family name is
	// allowed to resolve to one of its installed tags (preferring :latest).
	if strings.IndexByte(want, ':') >= 0 {
		return ""
	}

	var matches []string
	for _, m := range models {
		name := m
		if i := strings.IndexByte(name, ':'); i >= 0 {
			name = name[:i]
		}
		if strings.EqualFold(name, want) {
			matches = append(matches, m)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	for _, m := range matches {
		if strings.HasSuffix(strings.ToLower(m), ":latest") {
			return m
		}
	}
	if len(matches) > 1 {
		return matches[0]
	}
	return ""
}

// resolveOllamaModel verifies that Ollama is reachable and returns the model to
// use. An empty or "auto" model picks an installed qwen model first, since
// that is the default target for this build.
func resolveOllamaModel(baseURL, requested string) (string, error) {
	models, err := listOllamaModels(baseURL)
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("Ollama at %s has no models installed\nPull one with 'ollama pull %s'", baseURL, defaultOllamaModel)
	}

	requested = strings.TrimSpace(requested)
	if requested != "" && !strings.EqualFold(requested, "auto") {
		if m := matchOllamaModel(models, requested); m != "" {
			return m, nil
		}
		return "", fmt.Errorf("Ollama model %q is not installed at %s\nInstalled models: %s\nPull it with: ollama pull %s",
			requested, baseURL, strings.Join(models, ", "), requested)
	}

	for _, m := range models {
		if strings.HasPrefix(strings.ToLower(m), "qwen") {
			return m, nil
		}
	}

	fmt.Fprintf(os.Stderr, "nm: warning: no qwen model installed; using %s (pull one with 'ollama pull %s')\n", models[0], defaultOllamaModel)
	return models[0], nil
}

func typableRunes(kb *vilKeyboard) []string {
	if kb == nil {
		// Without a .vil, accept the printable ASCII range.
		var out []string
		for r := rune(33); r <= 126; r++ {
			out = append(out, string(r))
		}
		out = append(out, " ")
		sort.Strings(out)
		return out
	}

	seen := map[rune]bool{}
	var out []string

	for l := range kb.layers {
		for r := range kb.layers[l].matrix {
			for c := range kb.layers[l].matrix[r] {
				a := kb.actionAt(l, r, c)
				for _, ch := range []rune{a.output, a.shifted} {
					if ch == 0 || ch == ' ' || !unicode.IsPrint(ch) || seen[ch] {
						continue
					}
					seen[ch] = true
					out = append(out, string(ch))
				}
			}
		}
	}

	out = append(out, " ")
	sort.Strings(out)
	return out
}

func sanitizeGenLesson(kb *vilKeyboard, l genLesson) (genLesson, error) {
	valid := func(r rune) bool {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return true
		}
		if kb == nil {
			return r >= 32 && r <= 126
		}
		_, ok := kb.findRune(r)
		return ok
	}

	seen := map[rune]bool{}
	var chars []string
	for _, s := range l.Chars {
		for _, r := range s {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				continue
			}
			if !valid(r) {
				return l, fmt.Errorf("Ollama returned unsupported character %q", r)
			}
			if !seen[r] {
				seen[r] = true
				chars = append(chars, string(r))
			}
		}
	}

	text := strings.TrimSpace(bookWhitespace.ReplaceAllString(l.Text, " "))
	for _, r := range text {
		if !valid(r) {
			return l, fmt.Errorf("Ollama returned unsupported character %q", r)
		}
	}

	if len(chars) == 0 && text == "" {
		return l, fmt.Errorf("Ollama returned an empty drill")
	}

	l.Chars = chars
	l.Text = text
	if l.Count <= 0 {
		l.Count = 50
	}
	if l.Count > 500 {
		l.Count = 500
	}
	return l, nil
}

func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		} else {
			s = strings.TrimPrefix(s, "```json")
			s = strings.TrimPrefix(s, "```")
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	if i := strings.IndexByte(s, '{'); i >= 0 {
		if j := strings.LastIndexByte(s, '}'); j >= i {
			s = s[i : j+1]
		}
	}
	return strings.TrimSpace(s)
}

func ollamaGenerate(baseURL, model, prompt, format string, options map[string]interface{}, think *bool) (string, error) {
	reqBody := ollamaGenerateRequest{
		Model:   model,
		Prompt:  prompt,
		Stream:  false,
		Format:  format,
		Think:   think,
		Options: options,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := normalizeOllamaURL(baseURL) + "/api/generate"
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach Ollama at %s: %v", baseURL, err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("Ollama could not find model %q: %s\nPull it with: ollama pull %s",
			model, strings.TrimSpace(string(body)), model)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var cr ollamaGenerateResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("invalid Ollama response: %v", err)
	}
	if cr.Error != "" {
		return "", fmt.Errorf("Ollama: %s", cr.Error)
	}
	return cr.Response, nil
}

func requestOllamaText(baseURL, model, system, user string, maxTokens int) (string, error) {
	think := false
	return ollamaGenerate(baseURL, model, system+"\n\n"+user, "",
		map[string]interface{}{"temperature": 0.8, "num_predict": maxTokens}, &think)
}

var typingPunct = strings.NewReplacer(
	"‘", "'", "’", "'",
	"“", "\"", "”", "\"",
	"–", "-", "—", "-",
	"…", "...",
)

func validateTypingText(kb *vilKeyboard, text string) error {
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if kb == nil {
			if r < 32 || r > 126 {
				return fmt.Errorf("Ollama returned unsupported character %q", r)
			}
			continue
		}
		if _, ok := kb.findRune(r); !ok {
			return fmt.Errorf("Ollama returned unsupported character %q", r)
		}
	}
	return nil
}

// requestOllamaBookSummary turns one book paragraph into a single short
// sentence suitable for a fast typing sprint.
func requestOllamaBookSummary(baseURL, model, paragraph string, kb *vilKeyboard) (string, error) {
	if r := []rune(paragraph); len(r) > 2000 {
		paragraph = string(r[:2000])
	}
	system := `You summarize a book paragraph for a fast typing sprint.
Return one short, natural English sentence of at most 20 words (about 160
characters). Keep concrete nouns and verbs from the paragraph. Use only
printable ASCII, straight quotes, and hyphens. No markdown, no quotation marks
around the sentence, no commentary, no extra sentences.`

	think := false
	text, err := ollamaGenerate(baseURL, model, system+"\n\nparagraph:\n"+paragraph, "",
		map[string]interface{}{"temperature": 0.3, "num_predict": 80}, &think)
	if err != nil {
		return "", err
	}
	text = typingPunct.Replace(text)
	text = strings.Join(strings.Fields(text), " ")
	text = strings.Trim(text, "\"'` ")
	text = clipTypingWords(text, bookSummaryMaxWords)
	text = clipTypingText(text, bookSummaryMaxRunes)
	if text == "" {
		return "", fmt.Errorf("Ollama returned an empty summary")
	}
	if err := validateTypingText(kb, text); err != nil {
		return "", err
	}
	return text, nil
}

func requestOllamaLesson(baseURL, model, prompt string, kb *vilKeyboard) (genLesson, error) {
	allowed := strings.Join(typableRunes(kb), "")
	layout := "builtin Colemak-DH"
	var layers []string
	hrm := "none"
	if kb != nil {
		layout = "loaded Vial layout"
		layers = make([]string, len(kb.layers))
		for i := range kb.layers {
			layers[i] = kb.layerName(i)
		}
		if legend := kb.hrmLegend(); legend != "" {
			hrm = legend
		}
	}

	system := `You generate typing practice drills for a Colemak-DH keyboard.
Return ONLY a JSON object with this shape:
{"chars":["!","@","a"],"text":"","count":50,"focus":"short reason"}
Use "chars" for short character drills or "text" for sentences/paragraphs.
Only use characters from allowed_chars. Prefer the layers and HRMs named in the request.
Never invent keycodes or layout features.`

	user := fmt.Sprintf(
		"layout: %s\nallowed_chars: %q\nlayers: %s\nhome_row_mods: %s\nuser_request: %s\ncount: %d",
		layout, allowed, strings.Join(layers, ", "), hrm, prompt, 50,
	)

	think := false
	content, err := ollamaGenerate(baseURL, model, system+"\n\n"+user, "json",
		map[string]interface{}{"temperature": 0.7}, &think)
	if err != nil {
		return genLesson{}, err
	}

	var lesson genLesson
	if err := json.Unmarshal([]byte(stripJSONFences(content)), &lesson); err != nil {
		return genLesson{}, fmt.Errorf("Ollama did not return valid lesson JSON: %v\n%s", err, content)
	}
	return sanitizeGenLesson(kb, lesson)
}

func generateLLMDrill(lesson genLesson, n, g int) func() []segment {
	if len(lesson.Chars) > 0 {
		chars := append([]string{}, lesson.Chars...)
		count := lesson.Count
		if count <= 0 {
			count = n
		}
		if count > 500 {
			count = 500
		}
		return func() []segment {
			segments := make([]segment, g)
			for i := range segments {
				segments[i] = segment{randomText(count, chars), ""}
			}
			return segments
		}
	}

	text := lesson.Text
	return func() []segment {
		return []segment{{text, ""}}
	}
}
