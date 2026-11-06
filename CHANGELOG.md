# Unreleased:
 - Added `-arcade` galaxy shooter mode. Word-ships fire their letters at you;
   type a falling letter to shoot it down and clear a ship's whole word for a
   bonus.
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
