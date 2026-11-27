% nm(1)

# NAME

nm - A terminal based typing test for Colemak-DH

# SYNOPSIS

usage: nm \[OPTION\]... \[FILE\]

# DESCRIPTION

  By default nm creates a test consisting of 50 randomly generated words from
  the top 1000 words in the English language. If provided with a path, nm will
  use the given file as input treating each paragraph as a separate segment of
  the test. The program will automatically keep track of your position in the
  file so subsequent invocations on the same path will place you at the most
  recent paragraph (-start 0 can be used to reset your position).  
  
  Arbitrary text can also be piped directly into the program to create a custom
  test. Each paragraph of the input is treated as a segment unless '-multi' is
  supplied in which case each paragraph is treated as a separate test. 

  While a test is running the Colemak-DH (matrix) layout is shown at the bottom
  of the screen. The key producing the next character is highlighted in green.
  Pressing the wrong key highlights that key's location in red. In each split
  half the four home keys (`arst` on the left, `oien` on the right) are shown
  in their real columns, with the thumb keys tucked in at the inner edge.

# OPTIONS

## Modes

-words  *WORDFILE*

: Specifies the file from which words are randomly drawn (default: 1000en).

-quotes *QUOTEFILE*

: Starts quote mode in which quotes are randomly drawn from the given file. The file should be JSON encoded and have the following form:

    [{"text": "foo", attribution: "bar"}]

-vil *FILE*

: Load a Vial `.vil` keymap. If omitted, `~/.config/nm/*.vil` is used when
  exactly one layout is installed there. While typing, the matrix and the layer required for the next character are shown. Green marks the key to press, dark green marks the layer or shift key to hold, and red marks the most recent wrong key.

-layernames *NAMES*

: Comma separated layer names used with **-vil**, for example `base,nav,mouse,button,media,num,sym,fun`. Without this option layers are called `base`, `layer 1`, and so on.

-vil-split *MODE*

: Override the automatic split detection for **-vil**. MODE is `auto`, `rows`, `cols`, or `none`.

-vil-info

: Print the parsed matrix and every Vial layer, then exit.

-drill

: With **-vil**, generate a drill from characters that require a non-base layer.

-hrm

: With **-vil**, drill shifted characters using your home-row-mod shift keys.

-layer-track

: Follow physical layer keys by polling the Vial raw HID switch matrix on Linux. Enabled by default; use **-layer-track=false** to disable. Set `NM_HIDRAW=/dev/hidrawN` to choose the device, which must be readable/writable by your user and unlocked in Vial. Momentary keys (MO, LT, OSL, LM, TT) count while held, holding a layer-tap key long enough (or pressing another key while it is down) activates its layer, and TG/TO/DF are tracked as toggles, with the highest active layer shown. A `TD(n)` tap-dance whose hold action is a layer key counts as that layer. When tracking stops or a test ends the display falls back to the layer required by the next character.

-books *DIR*

: Draw random paragraphs from `.txt`, `.md`, `.text`, or `.org` files in DIR. By default each sprint types a one-sentence Ollama summary (20 words max) of the paragraph, shown inside the Chatsubo panel. Paragraphs shorter than 80 characters are skipped.

-books-raw

: With **-books**, type the paragraph itself instead of the Ollama summary.

-gen *PROMPT*

: Ask a local Ollama model for a practice drill. Works with the built-in Colemak-DH layout; with **-vil**, generated characters are validated against the loaded layout.

-ollama *URL*

: Ollama API URL (default `http://localhost:11434`).

-model *NAME*

: Ollama model to use (default: auto, prefers an installed qwen model).

-chatsubo

: Show live Chatsubo commentary from Ollama in the box above the keyboard. The practice text is typed inside this box. Uses **-ollama** and **-model**.

-chatsubo-every *DURATION*

: Time between Chatsubo comments (default `2m`; for example `30s`).

## Word Mode

-n *GROUPSZ*

: Sets the number of words which constitute a group.

-g *NGROUPS*

: Sets the number of groups which constitute a test.

## File Mode
-start *PARAGRAPH*

: The offset of the starting paragraph, set this to 0 to reset progress on a given file.

## Aesthetics

Live stats

: A live WPM, accuracy and error-count readout is always shown (displaying 0
  before you start). With `-chatsubo` it sits on top of the panel; otherwise it
  appears above the text. The speed figure is colour-zoned and flashes whenever
  the reading changes.

: The readout ends with **LAST** and **BEST**, for example
  `▸ 62 WPM · 310 CPM · 97% · 1 ERR · LAST 79%95 · BEST 91%98`. **LAST** is the
  previous completed test (a test left with **esc** or an arrow key does not
  count). **BEST** is the highest WPM completed; it rises during a run as soon
  as that run passes it, and its digits invert while the run is at or above it.
  Both are recorded per mode, so **-t 60** and **-n 50** keep separate records,
  and they persist in `~/.local/share/nm/.scores` across runs. On the narrow
  Chatsubo panel **CPM** and then **ERR** are dropped to keep both figures on
  screen. Scores are not recorded with **-noreport** or in **-books** sprint
  mode.

-theme *THEMEFILE*

: The theme to use. 

-notheme

: Attempt to use the default terminal theme. This may produce odd results depending on the theme colours.

-blockcursor

: Use the default cursor style.

-bold

: Embolden typed text.

-w *N*

: The maximum line length in characters (default: fit the full keyboard width). This option is ignored if -raw is present.

## Test Parameters

-t *SECONDS*

: Terminate the test after the given number of seconds.

-noskip

: Disable word skipping when space is pressed.

-nobackspace

: Disable the backspace key.

-nohighlight

: Disable highlighting.

-highlight1

: Only highlight the current word.

-highlight2

: Only highlight the next word.

-highlightchars N

: Highlight only the next N characters, not counting spaces, instead of whole
  words. A fixed look-ahead is easier to train for speed. Use this on its own;
  it is not combined with -highlight1 or -highlight2.

## Scripting

-oneshot

: Automatically exit after a single run.

-noreport

: Don't show a report at the end of a test.

-csv

: Print CSV formatted results.

	Tests have the form:

	```
	test,[wpm],[cpm],[accuracy],[timestamp].
	```

	Mistakes have the form:

	```
	mistake,[word],[typed]
	```

-json

: Print the test output in JSON.

-raw

: Don't reflow STDIN text or show one paragraph at a time.  Note that line breaks
are determined exclusively by the input.  

-multi 

: Treat each input paragraph as a self contained test.

## Misc

**-list** *TYPE*\

    Lists internal resources of the given type. TYPE=[themes|quotes|words].

**-v**\

    Print the current version.

# VIAL KEYMAPS

  The `.vil` file is used directly; just pass its path. The **-layernames**
  values are matched by position to the layers in the file. A Miryoku-style
  5x3+3 matrix is rendered as a seven-column split board, the two halves on the
  same columns, with each half's thumb keys at its inner edge.

  Use **-vil-info** to print the parsed matrix and every layer, then exit. Both
  string and numeric QMK keycodes are accepted, and flat Sofle/Corne `LAYOUT`
  lists are arranged into the split matrix. While typing, **F1**/**F2** preview
  the next/previous layer and **F3** returns to automatic layer selection. A
  preview wins over a physically held layer, so browsing layers with the function
  keys is never ignored.

  Tap dances stored as `{"tap": ..., "hold": ...}` objects are understood, so a
  `TD(n)` whose hold action is `MO`, `LT`, `OSL`, or similar activates that layer
  in the same way a direct layer key does.

  Home Row Mods are recognized from `LCTL_T`, `LSFT_T`, `LALT_T`, `LGUI_T`,
  `MT(...)`, and their numeric equivalents. Keycaps show both parts, like Vial:
  the modifier tag (`C`=Ctrl, `A`=Alt, `G`=Gui, `S`=Shift) on the top border and
  the tap letter in the middle. Compact pill keycaps show `[C a]`, `[A r]`, etc.
  **-vil-info** prints the mapping, for example `HRM: a=Ctrl r=Alt s=Gui
  t=Shift`. Use **-hrm** to drill shifted characters using your HRM shift key.
  Ctrl/Alt/GUI chords are not verifiable as text in a terminal and would need a
  dedicated chord drill mode.

  For every character nm finds the cheapest key which produces it in the loaded
  keymap and displays that layer. If no **-layernames** are supplied, layers are
  classified from their contents (`num/sym`, `mouse`, `fn/media`, `fn/shift`,
  `fn/ctrl`, `nav`, `shift`, `ctrl`), and those names are used in guidance and
  Chatsubo tips. The sequence is highlighted on the matrix: the layer activator
  and shift key are dark green, the target key is green, and the hint text spells out
  the order, for example `hold LT2 (num/sym) + press 5`. The border of the key to
  press blinks green and white; the key keeps its normal background. The inactive
  half is dimmed. Only the ten resting keys — four finger home keys per hand
  plus one thumb key per hand — are tinted dim green. The eight finger homes are
  located by the base-layer `a r s t` and `n e i o` keys, so they stay correct
  even when the `.vil` matrix has an extra inner or outer column. Only the border
  of the key to press blinks green/white; the resting keys stay static with a dim
  green label and the same border as the other keys. The thumb home is the space thumb if the
  layer has one, otherwise the innermost thumb key. A layer or shift key on the opposite half stays highlighted in dark green,
  since Miryoku-style layers often use the opposite thumb. Shift guidance uses
  the opposite half from the key being pressed: if `p` is on the left half, the
  right half's shift is recommended. The status line under the matrix spells
  out the sequence, for example:

  ```
  hold LT2 (sym) + press !
  hold o (shift) + press p
  ```

  The matrix geometry and layer contents come from the `.vil` file, so Sofle,
  Corne, and other split Vial keymaps work without a separate board definition.
  The split is auto-detected: matrices whose left-half rows are followed by the
  right-half rows are split by rows, while full-width matrices are split by
  columns. If it guesses wrong, override it with **-vil-split rows**,
  **-vil-split cols**, or **-vil-split none**.

  The window is divided into three horizontal parts, top to bottom: the
  Chatsubo box, the keyboard, and the live stats. The practice text is typed
  inside the box: word-wrapped to the box's interior and padded three rows above
  and below and several columns in from the border, so it never hugs the frame
  and there is no separate text band. The box is exactly as wide as the keyboard
  (both splits plus the split gap), so the two share one rectangle. Every part
  is placed inside the rows it owns, so nothing spills into another part however
  small the terminal is, and the parts shrink in a fixed order as the window gets
  shorter: the box's padding and text lines first, then the equalizer, then the
  keyboard's caps.

  The matrix scales to the terminal. Caps are drawn as closed keycaps with a
  top border, a label and a bottom border when the rows allow, and caps never
  grow past five cells, so the board keeps the shape of a keyboard instead of a
  wall of squares. A window too narrow for closed caps draws the same two hands
  as single-line keys (a tag for the home row modifier, the key's first
  character otherwise), and a window too narrow even for that draws one block
  with the hands still separated. Terminal font size itself is controlled by
  your terminal emulator.

  The board is one contiguous grid with the keys touching: no spaces between
  caps, and only the split gap separates the two halves. Every grid position is
  a real key. A position the keymap leaves empty is still drawn as a keycap and
  lights up (dark green) when you press it, so the whole physical matrix is
  visible. The pressed key, and any held layer or shift key, is highlighted in
  dark green. The resting home keys `a r s t` and `n e i o` draw their letters in
  green. For a Vial layout each half is laid out from its own inner edge on seven
  columns, and the thumb clusters sit at the inner edge of their own half on a
  row of their own, a line below the letter rows. On a terminal too narrow for
  the split layout the whole keyboard is drawn as one block, with the halves
  still separated.

  Use **-drill** to generate a test from the characters which require a
  non-base layer. Any text file can also be used as a lesson:

  ```
  nm -vil ~/sofle.vil lesson.txt
  ```

## BOOKS

  Files ending in `.txt`, `.md`, `.text`, or `.org` under the given directory are
  loaded as practice sources:

  ```
  nm -books ./books
  ```

  Paragraphs shorter than 80 characters are skipped. Books can be combined with
  **-vil** so layer and HRM guidance stays active.

  Book mode uses Ollama by default: each sprint types a one-sentence summary
  (20 words max) of a random paragraph. The practice text is typed inside the
  Chatsubo panel, which sits above the keys in every mode. A
  small queue of summaries is prefetched before
  the UI starts (with a progress line) and refilled in the background, so the
  next sprint starts immediately. The blocking report screen is skipped so the
  sprints flow into each other. Use **-books-raw** to type the paragraph itself:

  ```
  nm -books ./books -books-raw
  ```

  Live **-chatsubo** commentary is disabled in book sprint mode because the
  panel shows the countdown and the sprint status instead of commentary.

## OLLAMA

  **-gen** asks a local Ollama model for a drill and validates every character
  against the loaded `.vil`:

  ```
  nm -vil ~/sofle.vil -layernames base,nav,media,num,sym,fun \
     -model qwen3:0.6b -gen "sym layer with HRM shift" -t 600
  ```

  The model must support Ollama's `/api/generate` endpoint. If it returns an
  unsupported character, nm refuses the drill instead of showing an impossible
  target.

  With **-chatsubo**, nm sends the last interval's WPM, accuracy, errors, a
  short typed excerpt, the current key guidance, and the loaded `.vil` to
  Ollama. The model studies every layer, layer activator, and HRM, then gives a
  concrete tip about doing something on a named layer. It keeps the last three
  tips in the prompt to avoid repeats. The window is three horizontal parts,
  top to bottom: the panel in the first, the keyboard in the second, the live
  equalizer and readout in the third. The practice text is typed inside the
  panel: word-wrapped to the panel's interior and padded three rows above and
  below and several columns in from the border, so it never hugs the frame. The
  panel is exactly as wide as the keyboard and shares its columns, so the block
  reads as one rectangle. Each part is drawn only inside its own rows. In sprint
  mode the same panel is the top band, with the countdown in its rule.
  also includes the target finger in plain terms (left/right pinky, ring,
  middle, index, thumb), and Chatsubo is instructed to name it and give
  ergonomic Miryoku/Colemak-DH advice using home-row, home-column, reach, layer,
  and thumb-key language. Guidance and commentary are clipped to the body
  capacity, so they cannot spill outside the box. The panel is drawn at any
  terminal width that leaves it a readable box; a narrower one is dropped and
  the keyboard keeps the full window.
  **-chatsubo-every** controls the period (default `2m`; use `30s` for a quick
  test).

  On Linux, nm reads the Vial raw HID switch matrix by default
  (**-layer-track**) and switches the displayed layout to the layer whose thumb
  key is held. This needs read/write access to the Vial `/dev/hidraw*` interface
  (set `NM_HIDRAW=/dev/hidrawN` to override auto-detection) and an unlocked Vial
  keyboard. While a layer key is held the matrix shows only that layer's own
  mappings (transparent keys show `▽`) and the hint keeps the key to press
  (`held: num/sym — press 5`); if the next character needs a different layer,
  the full hold sequence is shown instead. Use **-layer-track=false** to
  disable it.

  Without raw HID tracking (non-Linux, locked Vial, or no permission), the
  terminal only receives the text produced by the operating system, not the
  physical key or layer. In that fallback nm displays the layer required by the
  next character; use **F1**/**F2** to preview layers and **F3** to return to
  automatic selection. Non-text actions such as arrows, mouse buttons, and
  macros are shown on the matrix but cannot be part of a text drill.

# EXAMPLES

Practice 50 common English words:
```
nm
```

Practice 100 words in four groups:
```
nm -n 25 -g 4
```

Run a timed test and exit:
```
nm -t 60 -oneshot
```

Use a built-in quote list:
```
nm -quotes en
```

Use a text file. Flags must come before the positional file:
```
nm -start 0 ~/war_and_peace.txt
nm -raw ~/war_and_peace.txt
nm -multi ~/war_and_peace.txt
```

Practice random dictionary words from stdin:
```
shuf -n 40 /usr/share/dict/words | nm
```

Practice quotes fetched from an API and write CSV:
```
curl https://api.quotable.io/quotes |\
    jq '[.results[]|.text=.content|.attribution=.author][:2]' |\
    nm -quotes - -noreport -csv
```

Load a Vial layout:
```
VIL=~/keyboards/sofle.vil

nm -vil "$VIL"
nm -vil "$VIL" -layernames base,mouse,num,fn,shift,ctrl
nm -vil "$VIL" -vil-info
nm -vil "$VIL" -drill
nm -vil "$VIL" -hrm
nm -vil "$VIL" lesson.txt
```

Follow physical layer keys through Vial raw HID on Linux:
```
nm -vil "$VIL"
```

Disable physical layer tracking and use F1/F2 previews instead:
```
nm -vil "$VIL" -layer-track=false
```

Practice random book paragraphs and show the source filename:
```
nm -vil "$VIL" -books ./books
```

Generate a drill with Ollama:
```
nm -model qwen3:0.6b -gen "home row words"
nm -vil "$VIL" -model qwen3:0.6b -gen "sym layer, avoid numbers"
```

Add live layout-aware commentary:
```
nm -vil "$VIL" -books ./books \
    -chatsubo -chatsubo-every 1m -model qwen3.5:0.8b
```

Scripted runs:
```
nm -oneshot -noreport -t 60
nm -csv -oneshot >> ~/wpm.csv
nm -json -oneshot > result.json
nm -list themes
```

Boolean flags such as **-chatsubo** do not take a value. Use
**-chatsubo-every DURATION** to change the interval:
```
nm -chatsubo -chatsubo-every 30s
```

## Copy/paste blocks

Each block is meant to be pasted as-is. Set VIL once and every later block
works unchanged (or leave it unset to use the built-in Colemak-DH matrix):
```
VIL=~/keyboards/sofle.vil
```

Ordinary practice:
```
nm
nm -t 30
nm -quotes en
nm -words 200en -n 25 -g 4
```

Your own text. Flags go before the file:
```
nm notes.txt
nm -t 60 notes.txt
nm -start 0 notes.txt
nm -multi notes.txt
cat notes.txt | nm
```

Vial layout and drills:
```
nm -vil "$VIL" -vil-info
nm -vil "$VIL" -layernames base,mouse,num,fn,shift,ctrl
nm -vil "$VIL" -drill
nm -vil "$VIL" -hrm
nm -vil "$VIL" -layer-track=false
```

Books and Ollama:
```
nm -vil "$VIL" -books ./books
nm -vil "$VIL" -books ./books -books-raw
nm -model qwen3:0.6b -gen "home row words"
nm -vil "$VIL" -model qwen3:0.6b -gen "sym layer, avoid numbers"
```

Live Chatsubo coaching:
```
nm -vil "$VIL" -layernames base,mouse,num,fn,shift,ctrl \
    -chatsubo -chatsubo-every 1m -model qwen3.5:0.8b
```

Training aids:
```
nm -highlightchars 3
nm -noskip
nm -nobackspace
nm -theme gruvbox
```

Piping results into other tools:
```
nm -oneshot -noreport -t 60
nm -csv -oneshot >> ~/wpm.csv
nm -json -oneshot | jq '.[].wpm'
shuf -n 40 /usr/share/dict/words | nm
```

Inspect the recorded LAST/BEST scores. They are kept per mode, so **-t 60**
and **-n 50** do not share a record:
```
cat ~/.local/share/nm/.scores | jq .
```

# PATHS

  Some options like **-words** and **-theme** accept a path. If the given path does
  not exist, the following directories are searched for a file with the given
  name before falling back to internal resources:

  ~/.nm/words\
  ~/.nm/themes\
  /etc/nm/words\
  /etc/nm/themes

# KEYS

  **esc: ** Restarts the test\
  **C-c: ** Terminates nm\
  **C-backspace: ** Deletes the previous word\
  **right** Move to the next test.\
  **left** Move to the previous test.\
  **F1/F2** Cycle through Vial layers.\
  **F3** Return to automatic layer selection.

# AUTHOR

Aetnaeus (aetnaeus@protonmail.com)

# SEE ALSO

## Project Page

    https://github.com/moresearch/neuromancer

# LICENSE

MIT
