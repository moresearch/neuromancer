package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell"
	"github.com/mattn/go-isatty"
)

var scr tcell.Screen
var csvMode bool
var jsonMode bool

type result struct {
	Wpm       int       `json:"wpm"`
	Cpm       int       `json:"cpm"`
	Accuracy  float64   `json:"accuracy"`
	Timestamp int64     `json:"timestamp"`
	Mistakes  []mistake `json:"mistakes"`
}

func die(format string, args ...interface{}) {
	if scr != nil {
		scr.Fini()
	}
	fmt.Fprintf(os.Stderr, "nm: ")
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func warnf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "nm: warning: "+format+"\n", args...)
}

func usageError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "nm: "+format+"\n", args...)
	fmt.Fprintln(os.Stderr, "Try 'nm -h' for usage.")
	os.Exit(2)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

var results []result

func parseConfig(b []byte) map[string]string {
	if b == nil {
		return nil
	}

	cfg := map[string]string{}
	for _, ln := range bytes.Split(b, []byte("\n")) {
		a := strings.SplitN(string(ln), ":", 2)
		if len(a) == 2 {
			cfg[a[0]] = strings.Trim(a[1], " ")
		}
	}

	return cfg
}

func exit(rc int) {
	scr.Fini()

	if jsonMode {
		//Avoid null in serialized JSON.
		for i := range results {
			if results[i].Mistakes == nil {
				results[i].Mistakes = []mistake{}
			}
		}

		b, err := json.Marshal(results)
		if err != nil {
			panic(err)
		}
		os.Stdout.Write(b)
	}

	if csvMode {
		for _, r := range results {
			fmt.Printf("test,%d,%d,%.2f,%d\n", r.Wpm, r.Cpm, r.Accuracy, r.Timestamp)
			for _, m := range r.Mistakes {
				fmt.Printf("mistake,%s,%s\n", m.Word, m.Typed)
			}
		}
	}

	os.Exit(rc)
}

func showReport(scr tcell.Screen, cpm, wpm int, accuracy float64, attribution string, mistakes []mistake) {
	mistakeStr := ""
	if attribution != "" {
		attribution = "\n\nAttribution: " + attribution
	}

	if len(mistakes) > 0 {
		mistakeStr = "\nMistakes:    "
		for i, m := range mistakes {
			mistakeStr += m.Word
			if i != len(mistakes)-1 {
				mistakeStr += ", "
			}
		}
	}

	report := fmt.Sprintf("WPM:         %d\nCPM:         %d\nAccuracy:    %.2f%%%s%s", wpm, cpm, accuracy, mistakeStr, attribution)

	scr.Clear()
	drawStringAtCenter(scr, report, tcell.StyleDefault)
	scr.HideCursor()
	scr.Show()

	for {
		if key, ok := scr.PollEvent().(*tcell.EventKey); ok && key.Key() == tcell.KeyEscape {
			return
		} else if ok && key.Key() == tcell.KeyCtrlC {
			exit(1)
		}
	}
}

func createDefaultTyper(scr tcell.Screen) *typer {
	return NewTyper(scr, true, tcell.ColorDefault,
		tcell.ColorDefault,
		tcell.ColorWhite,
		tcell.ColorGreen,
		tcell.ColorGreen,
		tcell.ColorMaroon)
}

func createTyper(scr tcell.Screen, bold bool, themeName string) *typer {
	var theme map[string]string

	if b := readResource("themes", themeName); b == nil {
		die("%s does not appear to be a valid theme, try '-list themes' for a list of built in thems.", themeName)
	} else {
		theme = parseConfig(b)
	}

	var bgcol, fgcol, hicol, hicol2, hicol3, errcol tcell.Color
	var err error

	if bgcol, err = newTcellColor(theme["bgcol"]); err != nil {
		die("bgcol is not defined and/or a valid hex colour.")
	}
	if fgcol, err = newTcellColor(theme["fgcol"]); err != nil {
		die("fgcol is not defined and/or a valid hex colour.")
	}
	if hicol, err = newTcellColor(theme["hicol"]); err != nil {
		die("hicol is not defined and/or a valid hex colour.")
	}
	if hicol2, err = newTcellColor(theme["hicol2"]); err != nil {
		die("hicol2 is not defined and/or a valid hex colour.")
	}
	if hicol3, err = newTcellColor(theme["hicol3"]); err != nil {
		die("hicol3 is not defined and/or a valid hex colour.")
	}
	if errcol, err = newTcellColor(theme["errcol"]); err != nil {
		die("errcol is not defined and/or a valid hex colour.")
	}

	return NewTyper(scr, bold, fgcol, bgcol, hicol, hicol2, hicol3, errcol)
}

var usage = `usage: nm [options] [file]

Modes
    -words  WORDFILE    Specifies the file from which words are randomly
                        drawn (default: 1000en).
    -quotes QUOTEFILE   Starts quote mode in which quotes are randomly drawn
                        from the given file. The file should be JSON encoded and
                        have the following form:

                        [{"text": "foo", attribution: "bar"}]
    -vil FILE           Load a Vial (.vil) keymap. The layout and the layer
                        required for the next character are shown while typing.
    -layernames NAMES   Comma separated layer names for -vil, e.g.
                        base,nav,media,num,sym,fun.
    -vil-split MODE     Split the Vial matrix into halves: auto, rows, cols
                        or none (default: auto).
    -vil-info           Print the parsed Vial layers and exit.
    -arcade             Galaxy arcade: word-ships fire letters at you. Type
                        the falling letters to shoot them down.
    -books DIR          Practice random paragraphs from .txt/.md/.text/.org
                        files in DIR.

Ollama (optional)
    -gen PROMPT         Generate a drill with a local Ollama model. Works with
                        the built-in Colemak-DH layout; add -vil FILE for
                        layout-aware drills.
    -chatsubo           Live typing commentary from a local Ollama model.
    -model NAME         Model to use (default: auto, prefers qwen).
                        Env: OLLAMA_MODEL
    -ollama URL         Ollama API URL (default: http://localhost:11434).
                        Env: OLLAMA_HOST
    -chatsubo-every DURATION
                        Time between comments (default: 2m; e.g. 30s).

Word Mode
    -n GROUPSZ          Sets the number of words which constitute a group.
    -g NGROUPS          Sets the number of groups which constitute a test.
    -drill              With -vil, generate a drill from characters that
                        require a non-base layer.
    -hrm                With -vil, drill shifted characters using your
                        home-row-mod shift keys.

File Mode
    -start PARAGRAPH    The offset of the starting paragraph, set this to 0 to
                        reset progress on a given file.
Aesthetics
    -showwpm            Display WPM whilst typing.
    -theme THEMEFILE    The theme to use. 
    -w N                The maximum line length in characters (default: 60).
                        Ignored if -raw is present.
    -notheme            Attempt to use the default terminal theme. 
                        This may produce odd results depending 
                        on the theme colours.
    -blockcursor        Use the default cursor style.
    -bold               Embolden typed text.
                        ignored if -raw is present.

The Colemak-DH (matrix) layout is shown at the bottom of the screen. The key
producing the next character is highlighted in green; pressing a wrong key
highlights that key's location in red. With -vil the loaded Vial layout is
shown instead, and the layer/shift key needed for the next character is
highlighted in cyan.

Test Parameters
    -t SECONDS          Terminate the test after the given number of seconds.
    -noskip             Disable word skipping when space is pressed.
    -nobackspace        Disable the backspace key.
    -nohighlight        Disable current and next word highlighting.
    -highlight1         Only highlight the current word.
    -highlight2         Only highlight the next word.

Scripting
    -oneshot            Automatically exit after a single run.
    -noreport           Don't show a report at the end of a test.
    -csv                Print the test results to stdout in the form:
                        [type],[wpm],[cpm],[accuracy],[timestamp].
    -json               Print the test output in JSON.
    -raw                Don't reflow STDIN text or show one paragraph at a time.
                        Note that line breaks are determined exclusively by the
                        input.
    -multi              Treat each input paragraph as a self contained test.

Misc
    -list TYPE          Lists internal resources of the given type.
                        TYPE=[themes|quotes|words]

Version
    -v                  Print the current version.

Examples
    nm
    nm -arcade
    nm -arcade -t 180
    nm -vil ~/sofle.vil
    nm -gen "home row words"
    nm -gen "sym layer with HRM shift" -vil ~/sofle.vil
    nm -chatsubo -model qwen3:0.6b
`

func saveMistakes(mistakes []mistake) {
	var db []mistake

	if err := readValue(MISTAKE_DB, &db); err != nil {
		db = nil
	}

	db = append(db, mistakes...)
	writeValue(MISTAKE_DB, db)
}

func main() {
	var n int
	var g int

	var rawMode bool
	var oneShotMode bool
	var noHighlightCurrent bool
	var noHighlightNext bool
	var noHighlight bool
	var maxLineLen int
	var noSkip bool
	var noBackspace bool
	var noReport bool
	var noTheme bool
	var normalCursor bool
	var timeout int
	var startParagraph int

	var listFlag string
	var wordFile string
	var quoteFile string
	var vilPath string
	var layerNames string
	var vilSplit string
	var vilInfo bool
	var booksDir string
	var genPrompt string
	var ollamaURL string
	var ollamaModel string

	var themeName string
	var showWpm bool
	var drillMode bool
	var hrmMode bool
	var arcadeMode bool
	var chatsuboMode bool
	var chatsuboEvery time.Duration
	var multiMode bool
	var versionFlag bool
	var boldFlag bool

	var err error
	var testFn func() []segment

	flag.IntVar(&n, "n", 50, "")
	flag.IntVar(&g, "g", 1, "")
	flag.IntVar(&startParagraph, "start", -1, "")

	flag.IntVar(&maxLineLen, "w", 60, "")
	flag.IntVar(&timeout, "t", -1, "")

	flag.BoolVar(&versionFlag, "v", false, "")

	flag.StringVar(&wordFile, "words", "", "")
	flag.StringVar(&quoteFile, "quotes", "", "")
	flag.StringVar(&vilPath, "vil", "", "")
	flag.StringVar(&layerNames, "layernames", "", "")
	flag.StringVar(&vilSplit, "vil-split", "auto", "")
	flag.BoolVar(&vilInfo, "vil-info", false, "")
	flag.StringVar(&booksDir, "books", "", "")
	flag.StringVar(&genPrompt, "gen", "", "")
	flag.StringVar(&ollamaURL, "ollama", normalizeOllamaURL(envOr("OLLAMA_HOST", defaultOllamaURL)), "")
	flag.StringVar(&ollamaModel, "model", envOr("OLLAMA_MODEL", ""), "")

	flag.BoolVar(&showWpm, "showwpm", false, "")
	flag.BoolVar(&arcadeMode, "arcade", false, "")
	flag.BoolVar(&drillMode, "drill", false, "")
	flag.BoolVar(&hrmMode, "hrm", false, "")
	flag.BoolVar(&chatsuboMode, "chatsubo", false, "")
	flag.DurationVar(&chatsuboEvery, "chatsubo-every", 2*time.Minute, "")
	flag.BoolVar(&noSkip, "noskip", false, "")
	flag.BoolVar(&normalCursor, "blockcursor", false, "")
	flag.BoolVar(&noBackspace, "nobackspace", false, "")
	flag.BoolVar(&noTheme, "notheme", false, "")
	flag.BoolVar(&oneShotMode, "oneshot", false, "")
	flag.BoolVar(&noHighlight, "nohighlight", false, "")
	flag.BoolVar(&noHighlightCurrent, "highlight2", false, "")
	flag.BoolVar(&noHighlightNext, "highlight1", false, "")
	flag.BoolVar(&noReport, "noreport", false, "")
	flag.BoolVar(&boldFlag, "bold", false, "")
	flag.BoolVar(&csvMode, "csv", false, "")
	flag.BoolVar(&jsonMode, "json", false, "")
	flag.BoolVar(&rawMode, "raw", false, "")
	flag.BoolVar(&multiMode, "multi", false, "")
	flag.StringVar(&themeName, "theme", "default", "")
	flag.StringVar(&listFlag, "list", "", "")

	flag.Usage = func() { os.Stdout.Write([]byte(usage)) }
	flag.Parse()

	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if listFlag != "" {
		switch listFlag {
		case "themes", "quotes", "words":
		default:
			usageError("unknown -list type %q (use themes, quotes or words)", listFlag)
		}
		prefix := listFlag + "/"
		var names []string
		for path := range packedFiles {
			if strings.HasPrefix(path, prefix) {
				_, f := filepath.Split(path)
				names = append(names, f)
			}
		}
		sort.Strings(names)
		for _, f := range names {
			fmt.Println(f)
		}
		os.Exit(0)
	}

	if versionFlag {
		fmt.Fprintf(os.Stderr, "nm version 0.4.2\n")
		os.Exit(0)
	}

	if n <= 0 {
		usageError("-n must be a positive number of words (got %d)", n)
	}
	if g <= 0 {
		usageError("-g must be a positive number of groups (got %d)", g)
	}
	if maxLineLen < 20 {
		usageError("-w must be at least 20 columns (got %d)", maxLineLen)
	}
	if timeout != -1 && timeout <= 0 {
		usageError("-t must be a positive number of seconds (got %d)", timeout)
	}
	if startParagraph < -1 {
		usageError("-start must be -1 or a paragraph number (got %d)", startParagraph)
	}
	if len(flag.Args()) > 1 {
		hint := ""
		for _, a := range flag.Args() {
			if strings.HasPrefix(a, "-") {
				hint = "\nHint: flags must appear before positional arguments, and boolean flags like -chatsubo take no value"
				break
			}
		}
		usageError("expected at most one input file, got %d%s", len(flag.Args()), hint)
	}

	if vilPath == "" {
		switch {
		case set["layernames"] && strings.TrimSpace(layerNames) != "":
			usageError("-layernames requires -vil FILE")
		case vilSplit != "auto":
			usageError("-vil-split requires -vil FILE")
		case vilInfo:
			usageError("-vil-info requires -vil FILE")
		}
	}
	if drillMode && vilPath == "" {
		usageError("-drill requires -vil FILE")
	}
	if hrmMode && vilPath == "" {
		usageError("-hrm requires -vil FILE")
	}

	var modes []string
	if arcadeMode {
		modes = append(modes, "-arcade")
	}
	if genPrompt != "" {
		modes = append(modes, "-gen")
	}
	if booksDir != "" {
		modes = append(modes, "-books")
	}
	if drillMode {
		modes = append(modes, "-drill")
	}
	if hrmMode {
		modes = append(modes, "-hrm")
	}
	if wordFile != "" {
		modes = append(modes, "-words")
	}
	if quoteFile != "" {
		modes = append(modes, "-quotes")
	}
	if len(flag.Args()) > 0 {
		modes = append(modes, "file argument")
	}
	if len(modes) > 1 {
		usageError("input modes cannot be combined: %s", strings.Join(modes, ", "))
	}
	if arcadeMode && chatsuboMode {
		usageError("-arcade cannot be combined with -chatsubo")
	}
	if startParagraph >= 0 && len(flag.Args()) == 0 {
		usageError("-start requires a file argument")
	}

	if genPrompt == "" && !chatsuboMode {
		if set["model"] {
			warnf("-model has no effect without -gen or -chatsubo")
		}
		if set["ollama"] {
			warnf("-ollama has no effect without -gen or -chatsubo")
		}
	}
	if !chatsuboMode && set["chatsubo-every"] {
		warnf("-chatsubo-every has no effect without -chatsubo")
	}
	if chatsuboMode && chatsuboEvery > 0 && chatsuboEvery < 5*time.Second {
		warnf("-chatsubo-every below 5s; using 5s")
	}

	if noTheme {
		os.Setenv("TCELL_TRUECOLOR", "disable")
	}

	var vilKB *vilKeyboard
	if vilPath != "" {
		vilKB, err = loadVil(vilPath, layerNames)
		if err != nil {
			die("Failed to load %s: %v", vilPath, err)
		}
		if err := vilKB.setSplitMode(vilSplit); err != nil {
			die("Invalid -vil-split: %v", err)
		}
		if vilInfo {
			fmt.Print(vilKB.info())
			os.Exit(0)
		}
	} else if vilSplit != "auto" || vilInfo {
		usageError("-vil-split and -vil-info require -vil FILE")
	}

	if genPrompt != "" || chatsuboMode {
		ollamaURL = normalizeOllamaURL(ollamaURL)
		resolved, err := resolveOllamaModel(ollamaURL, ollamaModel)
		if err != nil {
			die("%v", err)
		}
		ollamaModel = resolved
		fmt.Fprintf(os.Stderr, "nm: using Ollama model %s at %s\n", ollamaModel, ollamaURL)
	}

	switch {
	case arcadeMode:
		// Arcade mode is started after the tcell screen is initialized.
	case genPrompt != "":
		fmt.Fprintln(os.Stderr, "Generating lesson with Ollama...")
		lesson, err := requestOllamaLesson(ollamaURL, ollamaModel, genPrompt, vilKB)
		if err != nil {
			die("%v", err)
		}
		if lesson.Focus != "" {
			fmt.Fprintf(os.Stderr, "Focus: %s\n", lesson.Focus)
		}
		testFn = generateLLMDrill(lesson, n, g)
	case booksDir != "":
		testFn = generateBookTest(booksDir)
	case drillMode:
		if vilKB == nil {
			die("-drill requires -vil <file>")
		}
		testFn = generateVilDrill(vilKB, n, g)
		if testFn == nil {
			die("-drill: no layer-only printable characters found in %s", vilPath)
		}
	case hrmMode:
		if vilKB == nil {
			die("-hrm requires -vil <file>")
		}
		testFn = generateHrmDrill(vilKB, n, g)
		if testFn == nil {
			die("-hrm: no home-row-mod shift keys found in %s", vilPath)
		}
	case wordFile != "":
		testFn = generateWordTest(wordFile, n, g)
	case quoteFile != "":
		testFn = generateQuoteTest(quoteFile)
	case !isatty.IsTerminal(os.Stdin.Fd()):
		b, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}

		testFn = generateTestFromData(b, rawMode, multiMode)
	case len(flag.Args()) > 0:
		path := flag.Args()[0]
		testFn = generateTestFromFile(path, startParagraph)
	default:
		testFn = generateWordTest("1000en", n, g)
	}

	scr, err = tcell.NewScreen()
	if err != nil {
		panic(err)
	}

	if err := scr.Init(); err != nil {
		panic(err)
	}

	defer func() {
		if r := recover(); r != nil {
			scr.Fini()
			panic(r)
		}
	}()

	if arcadeMode {
		exit(runArcade(scr, timeout, oneShotMode))
	}

	var typer *typer
	if noTheme {
		typer = createDefaultTyper(scr)
	} else {
		typer = createTyper(scr, boldFlag, themeName)
	}

	if noHighlightNext || noHighlight {
		typer.currentWordStyle = typer.nextWordStyle
		typer.nextWordStyle = typer.defaultStyle
	}

	if noHighlightCurrent || noHighlight {
		typer.currentWordStyle = typer.defaultStyle
	}

	typer.SkipWord = !noSkip
	typer.DisableBackspace = noBackspace
	typer.BlockCursor = normalCursor
	typer.ShowWpm = showWpm
	typer.vil = vilKB
	if chatsuboMode {
		typer.chatsubo = newChatsubo(ollamaURL, ollamaModel, chatsuboEvery, chatsuboLayoutSummary(vilKB))
	}

	reflow := func(s string) string {
		sw, sh := scr.Size()
		top := sh
		kbAbove := false
		if _, kt, ok := typer.keyboardVisible(); ok {
			top = kt
			kbAbove = kt >= chatsuboMinAbove
		}
		// Leave room above the keyboard for the Chatsubo box and, when
		// present, the attribution/timer line below the text.
		maxLines := top - 2
		if chatsuboMode && kbAbove {
			maxLines = top - chatsuboBoxHeight - 2
			if timeout != -1 {
				maxLines--
			}
		}
		if maxLines < 1 {
			maxLines = 1
		}
		wsz := maxLineLen
		if sw > 8 {
			if wsz > sw-8 {
				wsz = sw - 8
			}
		} else {
			wsz = 1
		}
		if wsz < 1 {
			wsz = 1
		}
		return trimText(s, wsz, maxLines)
	}

	if timeout != -1 {
		timeout *= 1e9
	}

	var tests [][]segment
	var idx = 0

	for {
		if idx >= len(tests) {
			tests = append(tests, testFn())
		}

		if tests[idx] == nil {
			exit(0)
		}

		if !rawMode {
			for i, _ := range tests[idx] {
				tests[idx][i].Text = reflow(tests[idx][i].Text)
			}
		}

		nerrs, ncorrect, t, rc, mistakes := typer.Start(tests[idx], time.Duration(timeout))
		saveMistakes(mistakes)

		switch rc {
		case TyperNext:
			idx++
		case TyperPrevious:
			if idx > 0 {
				idx--
			}
		case TyperComplete:
			cpm := 0
			if t > 0 {
				cpm = int(float64(ncorrect) / (float64(t) / 60e9))
			}
			wpm := cpm / 5
			accuracy := 0.0
			if total := nerrs + ncorrect; total > 0 {
				accuracy = float64(ncorrect) / float64(total) * 100
			}

			results = append(results, result{wpm, cpm, accuracy, time.Now().Unix(), mistakes})
			if !noReport {
				attribution := ""
				if len(tests[idx]) == 1 {
					attribution = tests[idx][0].Attribution
				}
				showReport(scr, cpm, wpm, accuracy, attribution, mistakes)
			}
			if oneShotMode {
				exit(0)
			}

			idx++
		case TyperSigInt:
			exit(1)

		case TyperResize:
			//Resize events restart the test, this shouldn't be a problem in the vast majority of cases
			//and allows us to avoid baking rewrapping logic into the typer.

			//TODO: implement state-preserving resize (maybe)
		}
	}
}
