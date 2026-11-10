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
  Pressing the wrong key highlights that key's location in red.

# OPTIONS

## Modes

-words  *WORDFILE*

: Specifies the file from which words are randomly drawn (default: 1000en).

-quotes *QUOTEFILE*

: Starts quote mode in which quotes are randomly drawn from the given file. The file should be JSON encoded and have the following form:

    [{"text": "foo", attribution: "bar"}]

-vil *FILE*

: Load a Vial `.vil` keymap. While typing, the matrix and the layer required for the next character are shown. Green marks the key to press, cyan marks the layer or shift key to hold, and red marks the most recent wrong key.

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

: Follow physical layer keys by polling the Vial raw HID switch matrix on Linux. Enabled by default; use **-layer-track=false** to disable. Set `NM_HIDRAW=/dev/hidrawN` to choose the device, which must be readable/writable by your user and unlocked in Vial.

-books *DIR*

: Draw random paragraphs from `.txt`, `.md`, `.text`, or `.org` files in DIR. Paragraphs shorter than 80 characters are skipped.

-gen *PROMPT*

: Ask a local Ollama model for a practice drill. Works with the built-in Colemak-DH layout; with **-vil**, generated characters are validated against the loaded layout.

-ollama *URL*

: Ollama API URL (default `http://localhost:11434`).

-model *NAME*

: Ollama model to use (default: auto, prefers an installed qwen model).

-chatsubo

: Show live Chatsubo commentary from Ollama in a box between the practice text and the keyboard. Uses **-ollama** and **-model**.

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

-showwpm

: Display WPM whilst typing.

-theme *THEMEFILE*

: The theme to use. 

-notheme

: Attempt to use the default terminal theme. This may produce odd results depending on the theme colours.

-blockcursor

: Use the default cursor style.

-bold

: Embolden typed text.

-w *N*

: The maximum line length in characters (default: 60). This option is ignored if -raw is present.

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
  values are matched by position to the layers in the file. For a 60-key Sofle
  layout the matrix is rendered as a split 6x4 keyboard with five thumb keys
  per hand.

  Use **-vil-info** to print the parsed matrix and every layer, then exit. Both
  string and numeric QMK keycodes are accepted, and flat Sofle/Corne `LAYOUT`
  lists are arranged into the split matrix. While typing, **F1**/**F2** preview
  the next/previous layer and **F3** returns to automatic layer selection.

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
  and shift key are cyan, the target key is green, and the hint text spells out
  the order, for example `hold LT2 (num/sym) + press 5`. The border of the key to
  press blinks green and white; the key keeps its normal background. The inactive
  half is dimmed. Only the ten resting keys — four finger home keys per hand
  plus one thumb key per hand — are tinted dim green. The eight finger homes are
  located by the base-layer `a r s t` and `n e i o` keys, so they stay correct
  even when the `.vil` matrix has an extra inner or outer column. Only the border
  of the key to press blinks green/white; the resting keys stay static with a dim
  green label and the same border as the other keys. The thumb home is the space thumb if the
  layer has one, otherwise the innermost thumb key. A layer or shift key on the opposite half stays highlighted in cyan,
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

  The matrix scales to the terminal: it uses the largest key cells that fit,
  draws keys as beveled button keycaps, keeps a wide gap between the halves,
  and leaves padding below the keyboard. Terminal font size itself is
  controlled by your terminal emulator.

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

  Paragraphs shorter than 80 characters are skipped. The source filename is
  shown below the paragraph. Books can be combined with **-vil** so layer and
  HRM guidance stays active.

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
  tips in the prompt to avoid repeats. Text is wrapped to leave room for the
  box, so the two do not overlap. The full box has a fixed eight-row, 96-column
  size (shrinking only on narrower terminals), with two columns of horizontal
  padding and one blank padding row above and below the body. It shows the
  current layer/shift guidance line first (for example
  `hold o (shift) + press p`), followed by up to three wrapped lines of model
  commentary. Guidance and commentary are clipped to the body capacity, so they
  cannot spill outside the box. On short terminals a compact four-row version
  is used in the keyboard's bottom padding.
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
