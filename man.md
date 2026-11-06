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

-arcade

: Galaxy arcade mode. Words arrive as enemy ships and fire their letters at you. Type a falling letter to shoot it down; clear every letter of a ship's word to destroy it for a bonus. `R`/`Enter` restarts after game over and `Esc` quits.

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

-w 

: The maximum line length in characters (default: 60). This option is ignored if -raw is present.

## Test Parameters

-t *SECONDS*

: Terminate the test after the given number of seconds.

-noskip

: Disable word skipping when space is pressed.

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
  keymap and displays that layer. The border of the key to press blinks green and
  white; the key keeps its normal background. The inactive half is dimmed. Only the ten resting keys — four finger home keys per hand
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

## ARCADE

  **-arcade** is a self-contained galaxy shooter. Word-ships drift across the
  top of the screen and fire the letters of their word downward. Type the
  letters to shoot them down; let a letter reach your ship and you lose one of
  three hulls. Destroy every letter of a ship's word to blow the ship up. Waves add
  ships and speed as your score rises.

  ```
  nm -arcade
  nm -arcade -t 180
  ```

  **-t SECONDS** sets a time limit and **-oneshot** exits when the run ends.
  Arcade mode cannot be combined with the other input modes or **-chatsubo**.

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

  The terminal only receives the text produced by the operating system, not the
  physical key or layer. nm can verify the resulting text and teach which layer
  and key produce each character, but it cannot prove that the layer key was
  actually held. Pure layer keys such as `MO(...)`/`TG(...)` produce no terminal
  event, so nm cannot switch on a physical layer key press. It switches the
  displayed layer to the one required by the next character; use `F1`/`F2` to
  preview layers and `F3` to return to automatic selection. Real physical layer
  tracking needs raw HID/evdev input. Non-text actions such as arrows, mouse
  buttons, and macros are shown on the matrix but cannot be part of a text drill.

# EXAMPLES

Creates a series of tests each consisting of a random quote drawn from the
builtin quote file 'en'.
```
nm -quotes en
```

Creates a series of tests each consisting of 10 random words drawn from
words.txt
```
nm -words words.txt -n 10
```

Starts a sequence of tests in which each test consists of a paragraph from war
and peace starting with paragraph 1.
```
nm ~/war_and_peace.txt -start 1
```

Produces a test consisting of 40 random words draw from 
the system dictionary (similar to 'nm -n 40').
```
shuf -n 40 /usr/share/dict/words|nm
```

Starts a test consisting of two randomly drawn quotes from api.quotable.io and
prints the output of each test to STDOUT in csv format.
```
curl https://api.quotable.io/quotes|\
    jq '[.results[]|.text=.content|.attribution=.author][:2]'|\
    nm -quotes - -norreport -csv
```

Starts a new typing test which uses the nm source as input:

```
curl -LsS https://raw.githubusercontent.com/moresearch/neuromancer/master/src/nm.go | head -n 20 | nm -noskip -raw
```

Modify to taste.

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
