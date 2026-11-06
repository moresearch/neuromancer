# What

`nm` is a terminal based typing test for the Colemak-DH keyboard layout.

While a test is running the Colemak-DH (matrix) layout is shown at the bottom of
the screen. The key which produces the next character is highlighted in green,
and pressing the wrong key flashes that key's location in red.

![](demo.gif)

# Installation

## From source

```
# debian dependencies
sudo apt install golang

# clone and make
git clone https://github.com/moresearch/neuromancer
cd neuromancer
make && sudo make install
```

## Uninstall

```
sudo rm /usr/local/bin/nm /usr/share/man/man1/nm.1.gz
```

Best served on a terminal with truecolor and cursor shape support (e.g kitty, iterm)

# Usage

By default 50 words from the top 1000 words in the English language are used to
constitute the test. Custom text can be supplied by piping arbitrary text to the
program. Each paragraph in the input is shown as a separate segment of the text.
See `man nm` or `man.md` for a complete description and a comprehensive set of
options.

## Keys

- Pressing `escape` at any point restarts the test.
- `C-c` exits the test.
- `right` moves to the next test.
- `left` moves to the previous test.
- `F1` / `F2` cycle through the layers of a loaded `.vil` file.
- `F3` returns to the automatic layer for the next character.

## Examples

 - `nm -quotes en` Starts quote mode with the builtin quote list 'en'.
 - `nm -n 10 -g 5` produces a test consisting of 50 randomly drawn words in 5 groups of 10 words each.
 - `nm -t 10` starts a timed test lasting 10 seconds.
 - `nm -theme gruvbox` Starts nm with the gruvbox theme.

`nm` is designed to be easily scriptable and integrate nicely with
other *nix tools. With a little shell scripting most features the user can
conceive of should be possible to implement. Below are some simple examples of
what can be achieved.

 - `shuf -n 40 /usr/share/dict/words|nm`  Produces a test consisting of 40 random words drawn from your system's dictionary.
 - `curl http://api.quotable.io/random|jq '[.text=.content|.attribution=.author]'|nm -quotes -` Produces a test consisting of a random quote.
 - `alias nmd='nm -csv >> ~/wpm.csv'` Creates an alias called nmd which keeps a log of progress in your home directory`.

The default behaviour is equivalent to `nm -n 50`.

See `-help` for an exhaustive list of options.

## Vial keymaps and custom layers

`nm` reads a Vial `.vil` keymap directly. You do not need to copy it into the
repo; just pass the path:

```bash
nm -vil ~/keyboards/sofle.vil \
   -layernames base,nav,mouse,button,media,num,sym,fun
```

The `-layernames` values are matched by position to the layers in the `.vil`.
If you omit it, layers are shown as `base`, `layer 1`, and so on. For a 60-key
Sofle layout the matrix is rendered as a split 6x4 keyboard with the five thumb
keys per hand.

Use `-vil-info` to dump the parsed matrix and every layer, then exit:

```bash
nm -vil ~/keyboards/sofle.vil -layernames base,nav,mouse,button,media,num,sym,fun -vil-info
```

Both string keycodes (`KC_Q`, `MO(1)`, `LT(2,KC_SPC)`) and numeric QMK keycodes
are accepted. Flat Vial layouts in Sofle/Corne `LAYOUT` order are also arranged
into the split matrix. While typing, `F1`/`F2` preview the next/previous layer
and `F3` returns to automatic layer selection.

Home Row Mods are recognized from `LCTL_T`, `LSFT_T`, `LALT_T`, `LGUI_T`,
`MT(...)`, and their numeric equivalents. The keycaps show both parts, like
Vial: the modifier tag on the top border and the tap letter in the middle.

```text
╭──╮ ╭C─╮ ╭A─╮ ╭G─╮ ╭S─╮
│` │ │a │ │r │ │s │ │t │
╰──╯ ╰──╯ ╰──╯ ╰──╯ ╰──╯
```

`C` = Ctrl, `A` = Alt, `G` = Gui, `S` = Shift. On compact pill keycaps you get
`[C a]`, `[A r]`, etc. `-vil-info` prints the mapping, for example:

```text
HRM: a=Ctrl r=Alt s=Gui t=Shift n=Ctrl e=Alt i=Gui o=Shift
```

Use `-hrm` to drill shifted characters using your HRM shift key:

```bash
nm -vil ~/keyboards/sofle.vil \
   -layernames base,nav,mouse,button,media,num,sym,fun \
   -hrm
```

This exercises the Shift HRM (uppercase letters and shifted symbols). Ctrl,
Alt, and GUI chords are not verifiable as text in a terminal — they often get
intercepted by the terminal or window manager, and the terminal cannot prove
which physical HRM key produced the modifier. A guided chord drill would need
its own input mode.

For every character `nm` finds the cheapest key that produces it in the loaded
keymap and displays that layer. The border of the key to press blinks green and
white; the key itself keeps the normal background. Cyan marks the layer or shift
key to hold, and red marks the most recent wrong key. The inactive half is
dimmed. Only the ten resting keys — four finger home keys per
hand plus one thumb key per hand — are tinted dim green. The eight finger homes
are located by the base-layer `a r s t` and `n e i o` keys, so they stay correct
even when the `.vil` matrix has an extra inner or outer column. Only the border
of the key to press blinks green/white; the resting keys stay static with a dim
green label and the same border as the other keys. The thumb home is the space thumb if the
layer has one, otherwise the innermost thumb key. A layer or shift key on the opposite half stays highlighted in cyan,
since Miryoku-style layers often use the opposite thumb. The status line under
the matrix spells out the sequence, for example:

```
hold LT2 (sym) + press !
```

`-layernames` is optional; without it layers are called `base`, `layer 1`, and
so on. The matrix geometry and layer contents come from the `.vil` file, so
Sofle, Corne, and other split Vial keymaps work without a separate board
definition. The split is auto-detected: matrices whose left-half rows are
followed by the right-half rows are split by rows, while full-width matrices
are split by columns. If it guesses wrong, override it with
`-vil-split rows`, `-vil-split cols`, or `-vil-split none`.

The matrix scales to the terminal: it uses the largest key cells that fit,
draws keys as beveled button keycaps, keeps a wide gap between the halves, and
leaves padding below the keyboard. Terminal font size itself is controlled by
your terminal emulator.

To drill only the characters that require a non-base layer, add `-drill`:

```
nm -vil ~/sofle.vil -layernames base,nav,mouse,button,media,num,sym,fun -drill
```

Any text file also works as a lesson, so `nm -vil ~/sofle.vil lesson.txt` walks
through the exact layer sequence for every character in that file.

Limitations: the terminal only receives the text your OS produces, not the
physical key or layer. `nm` can verify the resulting text and teach which layer
and key produce each character, but it cannot prove that you actually held the
layer key. Pure layer keys such as `MO(...)`/`TG(...)` produce no terminal event,
so `nm` cannot switch on a physical layer key press. It switches the displayed
layer to the one required by the next character; use `F1`/`F2` to preview layers
and `F3` to return to automatic selection. Real physical layer tracking needs raw
HID/evdev input. Non-text actions such as arrows, mouse buttons, and macros are
shown on the matrix but cannot be part of a text drill.

## Configuration

Custom themes and word lists can be defined in `~/.nm/themes` and `~/.nm/words`
and used in conjunction with the `-theme` and `-words` flags. A list of
preloaded themes and word lists can be found in `words/` and `themes/` and are
accessible by default using the respective flags.
