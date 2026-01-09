package stockpile

import "strconv"

// Eval represents a chess position evaluation from the Lichess database.
type Eval struct {
	// FEN is the position in Forsyth-Edwards Notation.
	FEN string

	// Depth is the search depth used to compute this evaluation.
	Depth int

	// Knodes is the number of kilo-nodes searched.
	Knodes int

	// PVs contains all principal variations from multi-PV analysis.
	// The first PV is the best line.
	PVs []PV
}

// PV represents a principal variation (line of play) from the engine.
type PV struct {
	// Centipawns is the evaluation in centipawns from White's perspective.
	// Positive values favor White, negative values favor Black.
	// Nil if the position has a forced mate.
	Centipawns *int

	// Mate is the number of moves until checkmate.
	// Positive values mean White delivers mate, negative means Black.
	// Nil if there is no forced mate.
	Mate *int

	// Line is the sequence of moves in UCI notation.
	Line string
}

// BestPV returns the best principal variation, or nil if none available.
func (e *Eval) BestPV() *PV {
	if len(e.PVs) == 0 {
		return nil
	}
	return &e.PVs[0]
}

// IsMate returns true if the best line is a forced checkmate.
func (e *Eval) IsMate() bool {
	if pv := e.BestPV(); pv != nil {
		return pv.Mate != nil
	}
	return false
}

// Score returns a human-readable score string for the best line.
// Examples: "+1.25", "-0.50", "#3", "#-5"
func (e *Eval) Score() string {
	pv := e.BestPV()
	if pv == nil {
		return "?"
	}
	return pv.Score()
}

// Score returns a human-readable score string.
// Examples: "+1.25", "-0.50", "#3", "#-5"
func (pv *PV) Score() string {
	if pv.Mate != nil {
		return "#" + strconv.Itoa(*pv.Mate)
	}
	if pv.Centipawns == nil {
		return "?"
	}
	cp := *pv.Centipawns
	sign := "+"
	if cp < 0 {
		sign = "-"
		cp = -cp
	}
	whole := cp / 100
	frac := cp % 100
	if frac < 10 {
		return sign + strconv.Itoa(whole) + ".0" + strconv.Itoa(frac)
	}
	return sign + strconv.Itoa(whole) + "." + strconv.Itoa(frac)
}

// IsMate returns true if this PV is a forced checkmate.
func (pv *PV) IsMate() bool {
	return pv.Mate != nil
}
