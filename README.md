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

## Configuration

Custom themes and word lists can be defined in `~/.nm/themes` and `~/.nm/words`
and used in conjunction with the `-theme` and `-words` flags. A list of
preloaded themes and word lists can be found in `words/` and `themes/` and are
accessible by default using the respective flags.
