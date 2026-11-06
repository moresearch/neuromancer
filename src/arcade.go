package main

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gdamore/tcell"
)

// Galaxy arcade mode: enemy ships carry words and fire the letters of those
// words at the player. Type a falling letter to shoot it down. Destroy every
// letter fired by a ship to blow the ship up for a bonus.
const (
	arcadeTick          = 100 * time.Millisecond
	arcadeTicksPerSec   = 10
	arcadeStartLives    = 3
	arcadeScorePerWave  = 750
	arcadeShipFireFirst = 10
)

var (
	arcadeBaseStyle = tcell.StyleDefault.
			Background(tcell.ColorBlack).
			Foreground(tcell.ColorSilver)
	arcadeHudStyle = tcell.StyleDefault.
			Background(tcell.ColorBlack).
			Foreground(tcell.ColorAqua).
			Bold(true)
	arcadeLifeStyle = tcell.StyleDefault.
			Background(tcell.ColorBlack).
			Foreground(tcell.ColorRed).
			Bold(true)
	arcadeComboStyle = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.ColorFuchsia).
				Bold(true)
	arcadeMessageStyle = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.ColorYellow).
				Bold(true)
	arcadeShipStyle = tcell.StyleDefault.
			Background(tcell.ColorBlack).
			Foreground(tcell.ColorLime).
			Bold(true)
	arcadeShipSpentStyle = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.ColorGray)
	arcadeShipBorderStyle = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.ColorTeal)
	arcadeBulletStyle = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.ColorYellow).
				Bold(true)
	arcadeBulletTrailStyle = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.ColorOlive)
	arcadePlayerStyle = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.ColorAqua).
				Bold(true)
	arcadeDangerStyle = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.ColorRed).
				Bold(true)
	arcadeHelpStyle = tcell.StyleDefault.
			Background(tcell.ColorBlack).
			Foreground(tcell.ColorGray)
	arcadePanelStyle = tcell.StyleDefault.
				Background(tcell.ColorNavy).
				Foreground(tcell.ColorWhite)
	arcadePanelBorderStyle = tcell.StyleDefault.
				Background(tcell.ColorNavy).
				Foreground(tcell.ColorAqua).
				Bold(true)
)

var arcadeStarStyles = []tcell.Style{
	tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorNavy),
	tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGray),
	tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorSilver),
	tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite),
}

type arcadeStar struct {
	x, y  int
	ch    rune
	style tcell.Style
}

type arcadeShip struct {
	word      string
	x, y      int
	vx        int
	fired     int
	killed    int
	fireTicks int
}

type arcadeBullet struct {
	ch   rune
	ship *arcadeShip
	x, y int
	move int
}

type arcadeGame struct {
	scr   tcell.Screen
	rng   *rand.Rand
	words []string

	ships   []*arcadeShip
	bullets []*arcadeBullet
	stars   []arcadeStar

	score      int
	lives      int
	wave       int
	combo      int
	ticks      int
	spawnTicks int

	timeLimit int
	gameOver  bool
	endTitle  string

	message      string
	messageTicks int
}

func arcadeMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func arcadeWords() []string {
	fallback := []string{
		"laser", "orbit", "nova", "comet", "pulse", "cyber", "void",
		"star", "drift", "sprawl", "razor", "neon", "chrome", "matrix",
		"signal", "vector", "droid", "quasar", "nebula", "reactor",
	}

	b := readResource("words", "1000en")
	if b == nil {
		return fallback
	}

	seen := map[string]bool{}
	var out []string
	for _, w := range strings.Fields(strings.ToLower(string(b))) {
		if len(w) < 3 || len(w) > 9 || seen[w] {
			continue
		}
		ok := true
		for _, r := range w {
			if r < 'a' || r > 'z' {
				ok = false
				break
			}
		}
		if ok {
			seen[w] = true
			out = append(out, w)
		}
	}
	if len(out) < 20 {
		return fallback
	}
	return out
}

func newArcadeGame(scr tcell.Screen, timeLimit int) *arcadeGame {
	g := &arcadeGame{
		scr:       scr,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
		words:     arcadeWords(),
		lives:     arcadeStartLives,
		wave:      1,
		timeLimit: timeLimit,
	}
	g.seedStars()
	g.spawnShip()
	g.spawnTicks = g.spawnDelay()
	g.setMessage("WAVE 1", 30)
	return g
}

func (g *arcadeGame) seedStars() {
	sw, sh := g.scr.Size()
	g.stars = g.stars[:0]
	if sw <= 0 || sh <= 0 {
		return
	}
	n := sw * sh / 30
	if n < 20 {
		n = 20
	}
	glyphs := []rune{'.', '·', '*', '+'}
	for i := 0; i < n; i++ {
		g.stars = append(g.stars, arcadeStar{
			x:     g.rng.Intn(sw),
			y:     g.rng.Intn(sh),
			ch:    glyphs[g.rng.Intn(len(glyphs))],
			style: arcadeStarStyles[g.rng.Intn(len(arcadeStarStyles))],
		})
	}
}

func (g *arcadeGame) setMessage(msg string, ticks int) {
	g.message = msg
	g.messageTicks = ticks
}

func (g *arcadeGame) playerY() int {
	_, sh := g.scr.Size()
	y := sh - 2
	if y < 3 {
		y = 3
	}
	if y >= sh {
		y = sh - 1
	}
	if y < 0 {
		y = 0
	}
	return y
}

func (g *arcadeGame) maxShips() int {
	n := 2 + g.wave/3
	if n > 6 {
		n = 6
	}
	return n
}

func (g *arcadeGame) spawnDelay() int {
	d := 70 - g.wave*5
	if d < 25 {
		d = 25
	}
	return d + g.rng.Intn(21)
}

func (g *arcadeGame) fireDelay() int {
	d := 30 - g.wave*2
	if d < 10 {
		d = 10
	}
	return d + g.rng.Intn(16)
}

func (g *arcadeGame) bulletSpeed() int {
	s := 3 - g.wave/4
	if s < 1 {
		s = 1
	}
	return s
}

func (g *arcadeGame) spawnShip() {
	sw, sh := g.scr.Size()
	if sw < 12 || sh < 8 || len(g.words) == 0 {
		return
	}

	maxLen := sw - 6
	if maxLen > 9 {
		maxLen = 9
	}
	word := ""
	for tries := 0; tries < 40; tries++ {
		w := g.words[g.rng.Intn(len(g.words))]
		if len(w) <= maxLen {
			word = w
			break
		}
	}
	if word == "" {
		return
	}

	maxX := sw - len(word) - 2
	span := maxX - 2
	if span < 1 {
		span = 1
	}
	vx := 1
	if g.rng.Intn(2) == 0 {
		vx = -1
	}
	g.ships = append(g.ships, &arcadeShip{
		word:      word,
		x:         2 + g.rng.Intn(span),
		y:         2,
		vx:        vx,
		fireTicks: arcadeShipFireFirst + g.rng.Intn(20),
	})
}

func (g *arcadeGame) hasBullets(s *arcadeShip) bool {
	for _, b := range g.bullets {
		if b.ship == s {
			return true
		}
	}
	return false
}

func (g *arcadeGame) dropBullets(s *arcadeShip) {
	for i := len(g.bullets) - 1; i >= 0; i-- {
		if g.bullets[i].ship == s {
			g.bullets = append(g.bullets[:i], g.bullets[i+1:]...)
		}
	}
}

func (g *arcadeGame) removeShip(i int) {
	if i < 0 || i >= len(g.ships) {
		return
	}
	s := g.ships[i]
	g.ships = append(g.ships[:i], g.ships[i+1:]...)
	g.dropBullets(s)
}

func (g *arcadeGame) loseLife(msg string) {
	g.lives--
	g.combo = 0
	if g.lives < 0 {
		g.lives = 0
	}
	g.setMessage(msg, 40)
	if g.lives == 0 {
		g.end("GAME OVER")
	}
}

func (g *arcadeGame) end(title string) {
	g.gameOver = true
	g.endTitle = title
	g.messageTicks = 0
}

func (g *arcadeGame) restart() {
	g.ships = nil
	g.bullets = nil
	g.score = 0
	g.lives = arcadeStartLives
	g.wave = 1
	g.combo = 0
	g.ticks = 0
	g.spawnTicks = 0
	g.gameOver = false
	g.endTitle = ""
	g.seedStars()
	g.spawnShip()
	g.spawnTicks = g.spawnDelay()
	g.setMessage("WAVE 1", 30)
}

func (g *arcadeGame) typeRune(r rune) {
	if g.gameOver {
		return
	}
	r = unicode.ToLower(r)
	if r < 'a' || r > 'z' {
		return
	}

	target := -1
	for i, b := range g.bullets {
		if b.ch == r && (target < 0 || b.y > g.bullets[target].y) {
			target = i
		}
	}
	if target < 0 {
		g.combo = 0
		return
	}

	b := g.bullets[target]
	g.bullets = append(g.bullets[:target], g.bullets[target+1:]...)
	g.combo++
	pts := 10 + (g.combo-1)*2
	g.score += pts
	if b.ship != nil {
		b.ship.killed++
	}
	g.setMessage(fmt.Sprintf("+%d", pts), 15)
}

func (g *arcadeGame) tick() {
	if g.gameOver {
		return
	}
	g.ticks++
	if g.timeLimit > 0 && g.ticks >= g.timeLimit*arcadeTicksPerSec {
		g.end("TIME UP")
		return
	}
	if g.messageTicks > 0 {
		g.messageTicks--
	}

	wave := 1 + g.score/arcadeScorePerWave
	if wave != g.wave {
		g.wave = wave
		g.setMessage(fmt.Sprintf("WAVE %d", g.wave), 35)
	}

	if g.spawnTicks > 0 {
		g.spawnTicks--
	} else {
		if len(g.ships) < g.maxShips() {
			g.spawnShip()
		}
		g.spawnTicks = g.spawnDelay()
	}

	sw, _ := g.scr.Size()
	for i := len(g.ships) - 1; i >= 0; i-- {
		s := g.ships[i]

		if g.ticks%4 == 0 {
			s.x += s.vx
			minX := 2
			maxX := sw - len(s.word) - 2
			if maxX < minX {
				maxX = minX
			}
			if s.x < minX {
				s.x = minX
				s.vx = 1
			}
			if s.x > maxX {
				s.x = maxX
				s.vx = -1
			}
		}
		if g.ticks%45 == 0 {
			s.y++
		}

		if s.y >= g.playerY()-1 {
			g.removeShip(i)
			g.loseLife("SHIP COLLISION")
			if g.gameOver {
				return
			}
			continue
		}

		s.fireTicks--
		if s.fireTicks <= 0 && s.fired < len(s.word) {
			g.bullets = append(g.bullets, &arcadeBullet{
				ch:   rune(s.word[s.fired]),
				ship: s,
				x:    s.x + len(s.word)/2,
				y:    s.y + 1,
			})
			s.fired++
			s.fireTicks = g.fireDelay()
		}
	}

	for i := len(g.bullets) - 1; i >= 0; i-- {
		b := g.bullets[i]
		b.move++
		if b.move >= g.bulletSpeed() {
			b.move = 0
			b.y++
		}
		if b.y >= g.playerY() {
			g.bullets = append(g.bullets[:i], g.bullets[i+1:]...)
			g.loseLife("HULL HIT")
			if g.gameOver {
				return
			}
		}
	}

	g.resolveShips()
}

func (g *arcadeGame) resolveShips() {
	for i := len(g.ships) - 1; i >= 0; i-- {
		s := g.ships[i]
		if s.fired < len(s.word) || g.hasBullets(s) {
			continue
		}
		if s.killed >= len(s.word) {
			bonus := 50 + len(s.word)*10
			g.score += bonus
			g.setMessage(fmt.Sprintf("%s DESTROYED +%d", strings.ToUpper(s.word), bonus), 35)
		} else {
			g.setMessage(strings.ToUpper(s.word)+" ESCAPED", 30)
		}
		g.removeShip(i)
	}
}

func (g *arcadeGame) resize() {
	g.seedStars()
	sw, sh := g.scr.Size()
	maxY := g.playerY() - 2
	for _, s := range g.ships {
		maxX := sw - len(s.word) - 2
		if maxX < 2 {
			maxX = 2
		}
		if s.x < 2 {
			s.x = 2
		}
		if s.x > maxX {
			s.x = maxX
		}
		if s.y >= maxY {
			s.y = maxY
		}
		if s.y < 2 {
			s.y = 2
		}
	}
	for _, b := range g.bullets {
		if b.x < 0 {
			b.x = 0
		}
		if b.x >= sw {
			b.x = sw - 1
		}
		if b.y >= sh-1 {
			b.y = sh - 2
		}
	}
}

func (g *arcadeGame) drawCentered(y int, text string, style tcell.Style) {
	sw, sh := g.scr.Size()
	if y < 0 || y >= sh || sw <= 0 {
		return
	}
	r := []rune(text)
	if len(r) > sw {
		r = r[:sw]
	}
	x := (sw - len(r)) / 2
	drawString(g.scr, x, y, string(r), -1, style)
}

func (g *arcadeGame) drawShip(s *arcadeShip) {
	sw, sh := g.scr.Size()
	word := []rune(s.word)
	x := s.x
	maxX := sw - len(word) - 1
	if maxX < 1 {
		maxX = 1
	}
	if x < 1 {
		x = 1
	}
	if x > maxX {
		x = maxX
	}
	if s.y < 0 || s.y >= sh {
		return
	}

	for i, r := range word {
		style := arcadeShipStyle
		if i < s.fired {
			style = arcadeShipSpentStyle
		}
		g.scr.SetContent(x+i, s.y, r, nil, style)
	}
	if x-1 >= 0 {
		g.scr.SetContent(x-1, s.y, '⟨', nil, arcadeShipBorderStyle)
	}
	if x+len(word) < sw {
		g.scr.SetContent(x+len(word), s.y, '⟩', nil, arcadeShipBorderStyle)
	}
	engineX := x + len(word)/2
	if s.y+1 < g.playerY() && engineX < sw {
		g.scr.SetContent(engineX, s.y+1, '▾', nil, arcadeShipBorderStyle)
	}
}

func (g *arcadeGame) drawBullet(b *arcadeBullet) {
	sw, sh := g.scr.Size()
	if b.x < 0 || b.x >= sw || b.y < 0 || b.y >= sh {
		return
	}
	if b.y > 1 {
		g.scr.SetContent(b.x, b.y-1, '·', nil, arcadeBulletTrailStyle)
	}
	g.scr.SetContent(b.x, b.y, b.ch, nil, arcadeBulletStyle)
}

func (g *arcadeGame) drawPlayer() {
	sw, sh := g.scr.Size()
	if sw < 10 || sh < 5 {
		return
	}
	y := g.playerY()
	if y < 0 || y >= sh {
		return
	}
	style := arcadePlayerStyle
	if g.lives <= 1 {
		style = arcadeDangerStyle
	}
	drawString(g.scr, sw/2-2, y, "◄▲►", -1, style)
}

func (g *arcadeGame) drawBorder(x, y, w, h int, style tcell.Style) {
	if w < 2 || h < 2 {
		return
	}
	for i := 1; i < w-1; i++ {
		g.scr.SetContent(x+i, y, '─', nil, style)
		g.scr.SetContent(x+i, y+h-1, '─', nil, style)
	}
	for j := 1; j < h-1; j++ {
		g.scr.SetContent(x, y+j, '│', nil, style)
		g.scr.SetContent(x+w-1, y+j, '│', nil, style)
	}
	g.scr.SetContent(x, y, '╭', nil, style)
	g.scr.SetContent(x+w-1, y, '╮', nil, style)
	g.scr.SetContent(x, y+h-1, '╰', nil, style)
	g.scr.SetContent(x+w-1, y+h-1, '╯', nil, style)
}

func (g *arcadeGame) drawGameOver() {
	sw, sh := g.scr.Size()
	title := g.endTitle
	if title == "" {
		title = "GAME OVER"
	}
	lines := []string{
		title,
		"",
		fmt.Sprintf("SCORE %d", g.score),
		fmt.Sprintf("WAVE %d", g.wave),
		"",
		"R RESTART · ESC QUIT",
	}

	w := 0
	for _, l := range lines {
		if n := len([]rune(l)); n > w {
			w = n
		}
	}
	w += 4
	h := len(lines) + 2
	x := (sw - w) / 2
	y := (sh - h) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+w > sw {
		w = sw - x
	}
	if y+h > sh {
		h = sh - y
	}
	if w < 2 || h < 2 {
		return
	}

	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			g.scr.SetContent(xx, yy, ' ', nil, arcadePanelStyle)
		}
	}
	g.drawBorder(x, y, w, h, arcadePanelBorderStyle)

	start := y + 1 + (h-2-len(lines))/2
	if start < y+1 {
		start = y + 1
	}
	for i, l := range lines {
		yy := start + i
		if yy >= y+h-1 {
			break
		}
		r := []rune(l)
		if len(r) > w-2 {
			r = r[:w-2]
		}
		xx := x + 1 + (w-2-len(r))/2
		drawString(g.scr, xx, yy, string(r), -1, arcadePanelStyle)
	}
}

func (g *arcadeGame) draw() {
	sw, sh := g.scr.Size()
	g.scr.SetStyle(arcadeBaseStyle)
	g.scr.Clear()

	if sw < 20 || sh < 8 {
		g.drawCentered(arcadeMax(0, sh/2), "TERMINAL TOO SMALL", arcadeDangerStyle)
		g.scr.Show()
		return
	}

	for _, s := range g.stars {
		if s.x >= 0 && s.x < sw && s.y >= 0 && s.y < sh {
			g.scr.SetContent(s.x, s.y, s.ch, nil, s.style)
		}
	}

	for _, s := range g.ships {
		g.drawShip(s)
	}
	for _, b := range g.bullets {
		g.drawBullet(b)
	}
	g.drawPlayer()

	g.drawCentered(sh-1, "TYPE THE FALLING LETTERS · ESC QUIT", arcadeHelpStyle)

	drawString(g.scr, 2, 0, fmt.Sprintf("SCORE %06d", g.score), -1, arcadeHudStyle)
	g.drawCentered(0, fmt.Sprintf("WAVE %d", g.wave), arcadeHudStyle)
	lives := "LIVES 0"
	if g.lives > 0 {
		lives = "LIVES " + strings.Repeat("♥", g.lives)
	}
	lx := sw - len([]rune(lives)) - 2
	if lx < 0 {
		lx = 0
	}
	drawString(g.scr, lx, 0, lives, -1, arcadeLifeStyle)

	if g.combo > 1 {
		g.drawCentered(1, fmt.Sprintf("COMBO x%d", g.combo), arcadeComboStyle)
	}
	if g.messageTicks > 0 && g.message != "" {
		g.drawCentered(1, g.message, arcadeMessageStyle)
	}

	if g.gameOver {
		g.drawGameOver()
	}
	g.scr.Show()
}

// runArcade runs the galaxy arcade game until the player quits. It returns the
// process exit code.
func runArcade(scr tcell.Screen, timeLimit int, oneShot bool) int {
	g := newArcadeGame(scr, timeLimit)

	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(arcadeTick)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = scr.PostEvent(nil)
			}
		}
	}()
	defer func() {
		close(done)
		wg.Wait()
	}()

	for {
		g.draw()
		if g.gameOver && oneShot {
			return 0
		}

		ev := scr.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			scr.Sync()
			g.resize()
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEscape:
				return 0
			case tcell.KeyCtrlC:
				return 1
			case tcell.KeyEnter:
				if g.gameOver {
					g.restart()
				}
			case tcell.KeyRune:
				if g.gameOver {
					if ev.Rune() == 'r' || ev.Rune() == 'R' {
						g.restart()
					}
				} else {
					g.typeRune(ev.Rune())
				}
			}
		default:
			if !g.gameOver {
				g.tick()
			}
		}
	}
}
