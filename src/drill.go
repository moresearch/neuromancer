package main

import "unicode"

// layerDrillRunes returns printable characters whose best route in the loaded
// keymap goes through a non-base layer. Characters which need shift on that
// layer are skipped so the drill stays focused on layer switching.
func layerDrillRunes(kb *vilKeyboard) []string {
	seen := map[rune]bool{}
	var out []string

	for layer := 0; layer < len(kb.layers); layer++ {
		for row := range kb.layers[layer].matrix {
			for col := range kb.layers[layer].matrix[row] {
				a := kb.actionAt(layer, row, col)
				for _, r := range []rune{a.output, a.shifted} {
					if r == 0 || r == ' ' || !unicode.IsPrint(r) || seen[r] {
						continue
					}

					h, ok := kb.findRune(r)
					if !ok || h.layer == 0 || h.shift {
						continue
					}

					seen[r] = true
					out = append(out, string(r))
				}
			}
		}
	}

	return out
}

func generateVilDrill(kb *vilKeyboard, n, g int) func() []segment {
	chars := layerDrillRunes(kb)
	if len(chars) == 0 {
		return nil
	}

	return func() []segment {
		segments := make([]segment, g)
		for i := 0; i < g; i++ {
			segments[i] = segment{randomText(n, chars), ""}
		}
		return segments
	}
}

func hrmShiftRunes(kb *vilKeyboard) []string {
	candidates := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()_+{}|:\"<>?~")
	seen := map[rune]bool{}
	var out []string

	for _, r := range candidates {
		h, ok := kb.findRune(r)
		if !ok || !h.shift || h.shiftRow < 0 {
			continue
		}
		if a := kb.actionAt(h.layer, h.shiftRow, h.shiftCol); a.mods != "Shift" {
			continue
		}
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, string(r))
	}

	return out
}

func generateHrmDrill(kb *vilKeyboard, n, g int) func() []segment {
	chars := hrmShiftRunes(kb)
	if len(chars) < 2 {
		return nil
	}

	return func() []segment {
		segments := make([]segment, g)
		for i := 0; i < g; i++ {
			segments[i] = segment{randomText(n, chars), ""}
		}
		return segments
	}
}
