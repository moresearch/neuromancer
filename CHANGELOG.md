# Unreleased:
 - The layout uses the window's vertical space better: key caps grow to up to
   two rows and the leftover height is spent on a gap between the rows, so a
   tall terminal gets a clear board, capped at the block's share of the window
   height so it never dominates the screen. The Chatsubo box grows into the
   block's spare rows (up to four) and shrinks to fit short windows. The board, box and stats are now laid out so they never overlap, and
   narrow key caps use one-letter modifier labels (`LCT`→`C`, `LSF`→`S`) instead
   of truncating, and two-row caps keep their label row intact.
 - The board is one continuous grid, not a split keyboard: the two hands are
   separated by a fixed two blank columns, and the window never narrows that
   joiner to buy a wider cap. Chatsubo (and a book sprint) is a band above the
   keys in every mode, so sprint text is typed in its own band under the panel
   instead of in a panel between the hands.
 - Unwired `KC_NO` cells are no longer drawn as placeholder `.` keycaps; they
   are left as empty space. Tap-dance keys (`TD(n)`) keep their real meaning:
   Vial's five-element `[tap, hold, tap2, tap2hold, term]` form is now parsed,
   so F-key/macro dances show `LCT`/`LSF` instead of a generic `TD`, and a dance
   whose tap is `KC_NO` still renders (it is a wired double-tap key).
 - The keyboard matrix shows the four home keys (`arst` / `oien`) in their real
   columns, set off from the other keys of the row by most of a key width of
   blank space, so reaching for a key outside the home columns reads as a jump.
   Thumb keys are laid out from the keymap's own columns, evenly spaced and
   tucked in at the inner edge of each half, on a row drawn a line below the
   letter rows. The halves of a row-split Vial matrix are laid out from their
   own inner edges: the right half's home block, thumb keys and width are
   measured against the right half, so the same column number no longer lands
   in the same place on both halves. The edge macro/function columns and the
   top function row of each half are drawn a little apart from the central
   typing block (detected by content, so it follows whatever keys the keymap
   reserves for function/macro use), keeping the keys you actually type on
   visually separate from the ones you don't.
 - The window is now divided into three horizontal parts, top to bottom: the
   Chatsubo box, the keyboard, and the live stats (equalizer plus readout), with
   the practice text in its own band between the box and the keys. `keyboardMetrics`
   computes the three parts once (`layoutParts`), every band draws only inside
   its own rows, and `start()` no longer recomputes placement, so the text can
   never overwrite the box's border and the stats can never be clipped off the
   left edge. The box is exactly as wide as the board and follows it on narrow
   windows (it used to keep a fixed 60-column width and vanish below 100
   columns); the box is drawn whenever it has room for a readable frame.
   As the window shrinks the parts give way in a fixed order: the box's extra
   commentary, the equalizer, the box's body, then the text's second row. The
   spare rows of a short window now go to the box's body rather than to taller
   caps.
 - The keyboard is drawn in the shape of an ergo board (the Sofle that
   carries a Miryoku keymap) as one continuous grid: four letter rows and a
   thumb row per hand, both hands side by side with two blank columns between
   them and no blank line floating the
   function/macro row. Caps are closed keycaps (`╭─╮ │xy│ ╰─╯`, or a closed
   two-row cap with side borders), capped at five cells wide, with the outer
   macro column set apart; a window too narrow for
   closed caps draws the same hands as single-line keys instead of dropping the
   second hand. Labels are never blank: `F13`→`13`, `LT3`→`3`, `LCT`→`C`, and a home
   row modifier's tag replaces the tap letter when the cell has room only for
   one. A short window draws the thinnest caps that fit instead of a flat row of
   clipped labels, preferring three cells of label room over two. A two-row cap with
   one cell of label room (the border takes the other two) shows the label's
   first character rather than an empty key, so a bound home-row key like `s`
   no longer disappears between the side rules.
 - Holding a layer key now highlights the layer that appears and names the key
   to press on that layer (resolving transparent cells down to the base layer),
   instead of only showing the layer name.
 - Layer keys: a `TD(n)` tap-dance whose hold action is a layer key now
   activates its layer, `DF(n)` for `n > 0` selects layer `n` (it used to clear
   all toggles), layer numbers outside the layout are ignored, `F1`/`F2`
   previews take precedence over a physically held layer, and a held layer is
   cleared when tracking stops or a test ends.
 - Book mode now uses Ollama by default: each sprint types a one-sentence
   summary (20 words max) of a random paragraph. The text is typed in its own
   band under the Chatsubo panel, which is a band above the keys in every mode.
   A small summary queue is
   prefetched before the UI starts (with a progress line) and refilled in the
   background so sprints start immediately. The blocking report screen is
   skipped so sprints flow into each other. Use `-books-raw` to type the
   paragraph itself.
 - The practice text spans the full keyboard width (both hands plus the
   joiner) instead of stopping at 60 columns. The text area's edges line up
   with the outer keycaps, and the text/keyboard/stats block is centered as a
   whole.
 - Chatsubo is a band above the keys, exactly as wide as the board and sharing
   its columns, in the same three-part frame (panel, keys, stats). The board
   stays one continuous grid: the two blank columns between the hands are fixed
   and are never narrowed to buy a wider cap. Chatsubo now receives the target
   finger in plain terms (left/right pinky, ring, middle, index, thumb) and is
   instructed to give ergonomic Miryoku/Colemak-DH advice using home-row,
   home-column, reach, layer, and thumb-key language.
 - Use spare terminal width to widen the gap between key columns without
   shrinking the keycaps. The keyboard now dims every column except the home
   column of the finger which should reach the next key, and the target key
   blinks light green / dark green.
 - Documented the remaining flags in the plain-text usage summary and man page
   (`-drill`, `-hrm`, `-nobackspace`, and the highlighting toggles), and removed
   unused drawing and wrapping helpers.
 - Layer tracking no longer hides the target key. While a layer key is held
   the hint reads `held: num/sym — press 5`, and if the next character lives on
   another layer it keeps the full `hold LT4 … + press 5` sequence. Characters
   missing from the loaded layout now produce an explicit `no key for …` hint
   instead of a blank line.
 - Moved the Chatsubo box between the practice text and the keyboard, and
   reserve enough rows that text, attribution, and timers do not overlap it.
   The box now shows the layer/shift guidance line first, then the model
   commentary.
 - Chatsubo now studies the loaded `.vil` (layers, matrices, activators, and
   HRMs), keeps the last three tips to avoid repeats, and answers with concrete
   tips for using specific layers.
 - Layers without `-layernames` are classified from their contents (num/sym,
   mouse, fn/media, fn/shift, fn/ctrl, nav, shift, ctrl), and guidance spells
   out the highlighted hold/layer/shift sequence before the target key.
 - Fixed stale key labels and old hint text when the displayed layer changes;
   the matrix area and hint line are cleared before each redraw.
 - Added Linux raw HID layer tracking (`-layer-track`). nm polls the Vial switch
   matrix, follows held thumb layer keys in real time, and shows only that
   layer's own mappings (transparent keys are marked `▽`) while held. Requires
   access to `/dev/hidraw*` and an unlocked Vial keyboard; `NM_HIDRAW` selects
   the device.
 - Enlarged Chatsubo to a fixed 96x8 box with padded text and up to three
   wrapped commentary lines; a compact four-row version is used on short
   terminals. Guidance and model text are clipped to the body capacity.
 - Shift guidance now recommends a shift key on the opposite split half from
   the key being pressed, preferring the other hand's HRM shift.
 - Book mode now shows the source filename below the paragraph.
 - Fixed long unbroken tokens overflowing the configured practice width.
 - Fixed `-gen` leaving thinking enabled on reasoning models.
 - Fixed stale Chatsubo statistics after backspacing and zero-length tests.
 - Added `-books DIR` to practice random paragraphs from local text files.
 - Added `-gen PROMPT` to generate drills with a local Ollama model, plus
   `-ollama` and `-model` options. Generated characters are validated against
   the `.vil` layout.
 - Wrap practice text to a smaller default width (60 columns) and trim it to
   the area above the keyboard so long paragraphs cannot overlap the UI.
 - Added `-chatsubo` and `-chatsubo-every` for live Chatsubo
   Ollama commentary on the last typing interval. The comment box sits in the
   bottom padding; thinking models are called with `think:false` so they return
   text instead of burning the token budget on reasoning.
 - Documented the remaining Ollama ideas in the README roadmap.
 - Renamed the program from tt to nm.
 - Added the Colemak-DH (matrix) keyboard display. The key for the next
   character is highlighted in green and wrong keys flash red.
 - Added `-vil` to load Vial keymaps. The loaded layers are rendered and the
   layer/shift key needed for the next character is highlighted in cyan.
 - Split the keyboard display into halves, mark the active half with `L`/`R`,
   and dim the inactive half while typing.
 - Enlarged the keyboard display, widened key spacing and the split gap, and
   added bottom padding.
 - Draw keys as beveled button keycaps, removed the title, center divider, and
   L/R markers, and widened the split gap.
 - Tint only the ten resting keys (four finger homes per hand plus one thumb
   per hand) dim green.
 - Locate the eight finger homes from the base-layer `a r s t` / `n e i o`
   characters instead of assuming the extra Sofle column is on the outside.
 - Blink the key that should be pressed.
 - Replaced the blocky keycaps with clean rounded/pill keycaps.
 - Accept numeric QMK keycodes and flat Sofle/Corne `LAYOUT` lists in `.vil`
   files, and added `-vil-info` to dump the parsed layers.
 - Added `F1`/`F2` layer preview and `F3` to return to automatic layer selection.
 - Recognize Home Row Mods (`LCTL_T`, `LSFT_T`, `LALT_T`, `LGUI_T`, `MT(...)`)
   and print them in `-vil-info`.
 - Added `-hrm` to drill shifted characters using an HRM shift key.
 - Show HRM encoding Vial-style: modifier tag on the keycap top border, tap
   letter in the middle. Compact pills show `[C a]`. Widened the keycaps so it
   fits.
 - Enlarged the keycaps to three rows and packed them tightly, with the split
   gap and inter-key spacing roughly halved.
 - Only the border of the key to press blinks (green/white); no background fill
   and no finger-movement highlight.
 - Draw the ten resting keys with green labels and normal borders.
 - Documented that a terminal cannot observe pure layer-key presses; layer
   preview uses F1/F2 and the displayed layer follows the next character.
 - Configuration and state now live in `~/.nm` and `~/.local/share/nm`.

# 0.4.2:
  Added -notheme, -blockcursor and -bold.

# 0.4.0:
  Too numerous to list (see the man page)

  Highlights:
  
 - Added -quotes.
 - Added support for navigating between tests via right/left.
 - Now store the user's position within a file if one is specified.
 - Improved documentation.

# 0.3.0:
 - Added support for custom word lists (`-words).
 - `-theme` now accepts a path.
 - Added `~/.tt/themes` and `~/.tt/words`.
 - Scrapped ~/.ttrc in favour of aliases/flags.
 - Included more default word lists. (`-list words`)

# 0.2.2:
 - Modified -g to correspond to the number of groups rather than the group size.
 - Added -multi
 - Added -v
 - Changed the default behaviour to restart the currently generated test rather than generating a new one
 - Added a CHANGELOG :P
