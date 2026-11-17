package main

import "strconv"

// scoreRecord keeps the two figures shown beside the live readout: the last
// completed test and the best one seen.
type scoreRecord struct {
	LastWpm int     `json:"last_wpm"`
	LastAcc float64 `json:"last_acc"`
	BestWpm int     `json:"best_wpm"`
	BestAcc float64 `json:"best_acc"`
}

// scoreMode tags a record by test shape, so a 60s sprint PB does not get mixed
// up with a 5-word drill PB.
func scoreMode(cpmMode, words int) string {
	if cpmMode == -1 {
		return "words:" + strconv.Itoa(words)
	}
	return "time:" + strconv.Itoa(cpmMode) + "s"
}

// loadScore returns the record for mode out of the per-mode store.
func loadScore(path, mode string) scoreRecord {
	all := map[string]scoreRecord{}
	if err := readValue(path, &all); err != nil {
		return scoreRecord{}
	}
	return all[mode]
}

func writeScore(path string, all map[string]scoreRecord) {
	if err := writeValue(path, all); err != nil {
		warnf("could not save scores: %v", err)
	}
}

// recordScore folds a finished test into the stored best/last, returning the
// updated record. WPM wins; accuracy only breaks ties.
func recordScore(path, mode string, wpm int, acc float64) scoreRecord {
	all := map[string]scoreRecord{}
	if err := readValue(path, &all); err != nil || all == nil {
		all = map[string]scoreRecord{}
	}

	s := all[mode]
	s.LastWpm, s.LastAcc = wpm, acc
	if wpm > s.BestWpm || (wpm == s.BestWpm && acc > s.BestAcc) {
		s.BestWpm, s.BestAcc = wpm, acc
	}
	all[mode] = s
	writeScore(path, all)
	return s
}
