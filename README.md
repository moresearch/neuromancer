# nm

`nm` is a terminal typing trainer for Colemak-DH and Vial keyboards. It shows
the keyboard matrix while you type, highlights the key for the next character,
teaches layer and HRM sequences, and can pull practice text from word lists,
quotes, books, generated drills, or live coaching.

![](demo.gif)

## Features

- Word, quote, file, stdin, and random-book practice.
- Built-in Colemak-DH matrix with shifted-key guidance.
- Vial `.vil` support: layer matrices, HRMs, layer activators, automatic split
  detection, and inferred layer names (`num/sym`, `mouse`, `fn/media`, …).
- Physical layer tracking on Linux through the Vial raw HID interface: hold a
  thumb layer key and the display switches to that layer.
- HRM drills (`-hrm`) and non-base-layer drills (`-drill`).
- Optional Ollama integration:
  - `-gen` generates a practice drill from a prompt.
  - `-chatsubo` gives live, layout-aware typing tips in a narrow panel between
    the keyboard halves.
- Themes, JSON/CSV output, timed tests, and shell-friendly input.

## Install

### From source

```bash
# Debian/Ubuntu dependencies
sudo apt install golang git

git clone https://github.com/moresearch/neuromancer
cd neuromancer
make
sudo make install
```

The binary is `bin/nm` before installation. `make install` installs `nm` and its
man page under `/usr/local` by default. Use `PREFIX=/path make install` to
change the prefix.

### Uninstall

```bash
sudo make uninstall
```

Best served on a terminal with truecolor and cursor-shape support, such as
kitty, foot, wezterm, alacritty, or iTerm2.

## Quick start

```bash
# 50 common English words
nm

# 30-second timed test
nm -t 30

# quote mode (built-in English quotes)
nm -quotes en

# arbitrary text from a file or stdin
nm notes.txt
cat notes.txt | nm

# Vial layout
nm -vil ~/keyboards/sofle.vil

# Vial layout plus a local book folder
nm -vil ~/keyboards/sofle.vil -books ./books
```

Flags must come **before** a positional input file:

```bash
nm -t 60 notes.txt      # correct
nm notes.txt -t 60      # wrong: -t is treated as an extra file argument
```

Boolean flags such as `-chatsubo` do not take a value:

```bash
nm -chatsubo            # correct
nm -chatsubo 1          # wrong; use -chatsubo-every 1m for an interval
```

## Common recipes

### Practice text

```bash
nm                                  # 50 common English words
nm -n 25 -g 4                       # 100 words, 4 groups
nm -words 200en -n 40               # another built-in word list
nm -quotes en                       # built-in English quotes
nm lesson.txt                       # custom text file
nm -start 0 lesson.txt              # reset saved progress for a file
nm -raw lesson.txt                  # preserve input line breaks
nm -multi lesson.txt                # each paragraph as its own test
shuf -n 40 /usr/share/dict/words | nm
```

### Vial layout

```bash
VIL=~/keyboards/sofle.vil

nm -vil "$VIL" -vil-info
nm -vil "$VIL" -layernames base,mouse,num,fn,shift,ctrl
nm -vil "$VIL" -drill
nm -vil "$VIL" -hrm
nm -vil "$VIL" lesson.txt
nm -vil "$VIL" -layer-track=false
```

### Books and coaching

```bash
VIL=~/keyboards/sofle.vil

nm -vil "$VIL" -books ./books
nm -vil "$VIL" -books ./books \
  -chatsubo -chatsubo-every 1m -model qwen3.5:0.8b

nm -model qwen3:0.6b -gen "home row words"
nm -vil "$VIL" -model qwen3:0.6b -gen "sym layer, avoid numbers"
```

### Timed and scripted runs

```bash
nm -t 60                            # 60-second test
nm -oneshot -noreport -t 60         # exit after one timed test
nm -csv -oneshot >> ~/wpm.csv       # append results to a CSV log
nm -json -oneshot > result.json     # write JSON results
nm -list themes                     # list built-in themes
```

## Keyboard controls

| Key | Action |
|---|---|
| `Esc` | Restart the current test |
| `Ctrl-C` | Quit |
| `Right` / `Left` | Next / previous paragraph |
| `F1` / `F2` | Preview next / previous Vial layer |
| `F3` | Return to automatic layer selection |
| `Ctrl-W`, `Ctrl-Backspace`, `Alt-Backspace` | Delete the current word |
| `Backspace` | Delete one character |

## Practice modes

### Words

```bash
nm -words 1000en -n 25 -g 4
```

- `-words FILE` selects a word list. Built-ins include `1000en`, `fr`, `de`,
  `es`, and others; see `nm -list words`.
- `-n` is the number of words per group (default 50).
- `-g` is the number of groups (default 1).

### Quotes

```bash
nm -quotes en
```

Quote files are JSON arrays of `{"text": "...", "attribution": "..."}`.
See `nm -list quotes` for built-ins.

### Files and stdin

Each paragraph becomes a segment of the test:

```bash
nm lesson.txt
nm -start 0 lesson.txt      # reset saved progress for this file
nm -raw lesson.txt          # do not reflow or split into paragraphs
nm -multi lesson.txt        # each paragraph is its own test
```

### Books

Put `.txt`, `.md`, `.text`, or `.org` files in a directory. `nm` picks
random paragraphs and shows the source filename below the text:

```bash
nm -books ./books
```

Paragraphs shorter than 80 characters are skipped, which filters out headings
and page numbers. The `./books` directory is gitignored.

Book mode uses Ollama by default: it asks for a one-sentence summary (20 words
max) of a random paragraph and uses that as the sprint text. The text is shown
inside the Chatsubo panel between the keyboard halves. A small queue of
summaries is prefetched before the UI starts (with a progress line) and
refilled in the background, so the next sprint starts immediately. The blocking
report screen is skipped so the sprints flow into each other.

Use `-books-raw` to type the paragraph itself instead:

```bash
nm -books ./books -books-raw
```

Live `-chatsubo` commentary is disabled in book sprint mode because the panel
is the typing area.

### Vial keymaps

```bash
nm -vil ~/keyboards/sofle.vil
nm -vil ~/keyboards/sofle.vil -layernames base,mouse,num,fn,shift,ctrl
```

`-layernames` is optional. Without it, `nm` infers layer purposes from the
`.vil` contents:

| Content | Inferred name |
|---|---|
| digits + punctuation | `num/sym` |
| mouse buttons / movement | `mouse` |
| F-keys / media keys | `fn/media` |
| F-keys wrapped in Shift | `fn/shift` |
| F-keys wrapped in Ctrl | `fn/ctrl` |
| arrows / navigation | `nav` |
| dedicated Shift keys | `shift` |
| dedicated Ctrl keys | `ctrl` |

Useful Vial commands:

```bash
# Print every layer, inferred purpose, HRMs, activators, and matrix
nm -vil ~/keyboards/sofle.vil -vil-info

# Drill characters that need a non-base layer
nm -vil ~/keyboards/sofle.vil -drill

# Drill shifted characters using your HRM shift key
nm -vil ~/keyboards/sofle.vil -hrm

# Practice a text file while showing layer guidance
nm -vil ~/keyboards/sofle.vil lesson.txt
```

The bottom matrix highlights the sequence:

- **cyan** — layer activator and/or shift key to hold
- **green** — final target key
- **red** — most recent wrong key

The hint line or Chatsubo panel spells out the sequence:

```text
hold LT2 (num/sym) + press 5
hold o (shift) + press p
press a
```

Shift guidance deliberately uses the **opposite split half** from the target
key, so shifted characters are typed with the other hand.

#### Physical layer tracking

On Linux, `nm` can follow the physical layer keys by polling the Vial raw HID
switch matrix. This is enabled by default when a `.vil` is loaded:

```bash
nm -vil ~/keyboards/sofle.vil
```

While you hold a thumb layer key, the matrix switches to that layer immediately,
shows only that layer’s mappings, and the hint keeps the target key, for example
`held: num/sym — press 5`. If the next character lives on another layer, the
full sequence is still shown (`held: base — hold LT4 (num/sym) + press 5`).
Transparent keys are shown as `▽`. Releasing the key returns to automatic
target-layer selection. Characters that do not exist on the loaded layout are
reported as `no key for ";" in this layout` instead of a blank hint.

Requirements:

- Linux.
- A Vial firmware with the raw HID interface (usage page `0xFF60`, usage
  `0x61`).
- Read/write access to `/dev/hidraw*`. The Vial interface is auto-detected;
  set `NM_HIDRAW=/dev/hidrawN` if needed.
- The keyboard unlocked in Vial. If Vial is locked, `nm` warns and falls back
  to target-layer display.

Disable it with:

```bash
nm -vil ~/keyboards/sofle.vil -layer-track=false
```

If raw HID is unavailable, `F1`/`F2` still preview layers and `F3` returns to
automatic selection.

#### Layer-tap keys and space

A Vial key such as `LT(1,KC_SPC)` is a layer-tap key:

- quick tap → sends Space
- hold → activates layer 1

`nm` delays layer switching by about 180 ms for layer-tap keys so a normal
Space tap does not flash the mouse layer. Pressing another key while the layer
key is held activates the layer immediately. If you need Space to auto-repeat
or behave as a normal held key, map Space to plain `KC_SPC` in your firmware and
use a separate thumb key for the mouse layer.

## Ollama drills

`-gen` asks a local Ollama model for a drill:

```bash
nm -model qwen3:0.6b -gen "home row words"
nm -vil ~/keyboards/sofle.vil \
  -model qwen3:0.6b \
  -gen "sym layer symbols, no numbers" -t 600
```

Options:

- `-ollama URL` — Ollama API URL (default `http://localhost:11434`)
- `-model NAME` — model name; default is auto (prefers an installed qwen model)
- `-gen PROMPT` — generate a drill

With `-vil`, every generated character is validated against the loaded layout.
Without `-vil`, printable ASCII is accepted. The model never controls keycodes;
`nm` only uses the text it returns.

## Chatsubo

Chatsubo gives live, layout-aware commentary while you type:

```bash
nm -vil ~/keyboards/sofle.vil \
  -layernames base,mouse,num,fn,shift,ctrl \
  -chatsubo -chatsubo-every 1m \
  -model qwen3.5:0.8b
```

Each interval, `nm` sends the model:

- WPM, accuracy, errors, and a short typed excerpt
- the current key guidance and target
- the loaded `.vil` layer matrices, inferred layer names, activators, and HRMs
- the last three tips, so it does not repeat itself

The model gives concrete layer tips such as:

```text
Num/sym layer: hold LT2 with your left thumb and press w for @.
```

The panel sits in the widened gap between the keyboard halves (40 columns wide,
about three key rows high plus a few rows, with two columns of side padding). It
shows one guidance line and wrapped commentary, all clipped to the panel body.
Every request includes the target finger in plain terms (left/right pinky,
ring, middle, index, thumb), and Chatsubo is instructed to name the finger and
give ergonomic Miryoku/Colemak-DH advice using home-row, home-column, reach,
layer, and thumb-key language, without numeric finger codes. On terminals
narrower than 100 columns the panel is omitted and the keyboard keeps its normal
split gap.

Options:

- `-chatsubo` — enable the live panel
- `-chatsubo-every DURATION` — interval between comments (default `2m`)
- `-ollama URL` / `-model NAME` — local Ollama settings

## Aesthetics and behavior

```bash
nm -theme gruvbox
nm -notheme
nm -showwpm
nm -bold
nm -blockcursor
nm -w 72
nm -noskip
nm -nobackspace
nm -nohighlight
nm -highlight1
nm -highlight2
```

- `-theme FILE` selects a built-in or `~/.nm/themes/FILE` theme.
- `-notheme` uses terminal colors.
- `-showwpm` displays live WPM.
- `-bold` emboldens typed text.
- `-w N` wraps practice text to N columns (default: fit the full keyboard width; `-raw` ignores it).
- `-noskip` disables automatic word skipping on Space.
- `-nobackspace` disables Backspace.
- `-nohighlight` disables word highlighting; `-highlight1` and
  `-highlight2` keep only the current or next word highlighted.

Scripting:

```bash
nm -oneshot -noreport -t 30
nm -csv  >> ~/wpm.csv
nm -json > result.json
nm -v                                # print the version
```

`-csv` prints:
`test,wpm,cpm,accuracy,timestamp` and `mistake,word,typed` rows.
`-json` prints an array of result objects.

## Configuration

Custom themes and word lists live in:

```text
~/.nm/themes/
~/.nm/words/
```

Built-ins are available with `-theme` and `-words`; list them with:

```bash
nm -list themes
nm -list words
nm -list quotes
```

## Troubleshooting

### “expected at most one input file”

Put flags before the file argument:

```bash
nm -t 60 notes.txt
```

Boolean flags do not take values:

```bash
nm -chatsubo -chatsubo-every 1m
```

### Space behaves oddly

If Space is mapped as `LT(layer,KC_SPC)`, holding it accesses a layer and only
a tap emits Space. `nm` cannot change firmware behavior; remap Space to
`KC_SPC` and use a separate layer key if you need a normal repeating Space.

### Physical layer tracking unavailable

```text
nm: physical layer tracking unavailable: ...
```

Check:

1. Vial is unlocked.
2. `/dev/hidraw*` is readable/writable by your user.
3. No Vial GUI is holding the raw HID interface.
4. Try `NM_HIDRAW=/dev/hidrawN`.
5. Or use `-layer-track=false` and `F1`/`F2` for manual preview.

### Layer hint is wrong

Run:

```bash
nm -vil your.vil -vil-info
```

Check the inferred `purposes:` line and use `-layernames` to override names.
If shift guidance uses the wrong side, check the `.vil` split mode; force it
with `-vil-split rows`, `-vil-split cols`, or `-vil-split none`.

## Limitations

- The terminal alone cannot prove which physical key produced a character.
  Raw layer tracking is Linux-only and requires Vial raw HID.
- Ctrl/Alt/GUI chords are often consumed by the terminal or window manager and
  are not used as typed practice text.
- Non-text actions (arrows, mouse buttons, macros) appear on the matrix but are
  not part of text drills.
- Physical layer tracking reflects your keyboard’s electrical matrix. If your
  `.vil` layout and firmware matrix order differ, the held-layer view may map
  incorrectly.

## Roadmap

- [x] Natural-language drill compiler (`-gen`)
- [x] Live Chatsubo commentary (`-chatsubo`)
- [x] Physical layer tracking from Vial raw HID
- [ ] Adaptive drills from the mistake database and per-layer stats
- [ ] Post-session coach report with likely causes and next drills

## License

MIT/X Consortium License. See [LICENSE](LICENSE).
