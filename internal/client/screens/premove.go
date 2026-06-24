package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/notnil/chess"
)

// gameForColor returns a new *chess.Game with the same board position as pos
// but with color forced to move next. Uses FEN string manipulation.
func gameForColor(pos *chess.Position, color chess.Color) *chess.Game {
	parts := strings.Fields(pos.String())
	if len(parts) < 2 {
		return nil
	}
	if color == chess.White {
		parts[1] = "w"
	} else {
		parts[1] = "b"
	}
	opt, err := chess.FEN(strings.Join(parts, " "))
	if err != nil {
		return nil
	}
	return chess.NewGame(opt)
}

// buildPremoveSimGame returns a *chess.Game representing the position after
// applying prevPremoves (skipping opponent responses) with playerColor to move.
// Returns nil if any premove in the chain is illegal.
func buildPremoveSimGame(base *chess.Game, playerColor chess.Color, prevPremoves []string) *chess.Game {
	sim := gameForColor(base.Position(), playerColor)
	if sim == nil {
		return nil
	}
	for _, san := range prevPremoves {
		if err := sim.MoveStr(san); err != nil {
			return nil
		}
		// After our premove the turn flips to opponent; reset to playerColor.
		sim = gameForColor(sim.Position(), playerColor)
		if sim == nil {
			return nil
		}
	}
	return sim
}

// clearPremoves wipes the premove queue and all associated display state.
func (m *GameModel) clearPremoves() {
	m.premoves = nil
	m.premovePairs = nil
	m.premoveList.SetMoves(nil)
	m.board.ClearPremoves()
	m.board.ClearSelected()
	m.input.ClearSelection()
}

// tryAddPremove validates san against the current simulated chain position and,
// if legal and under the 10-premove cap, appends it to the queue. Returns true on success.
func (m *GameModel) tryAddPremove(san string) bool {
	if len(m.premoves) >= 10 {
		return false
	}
	sim := buildPremoveSimGame(m.chess, m.myColor, m.premoves)
	if sim == nil {
		return false
	}
	pos := sim.Position()
	notation := chess.AlgebraicNotation{}
	for _, mv := range sim.ValidMoves() {
		if notation.Encode(pos, mv) == san {
			m.premoves = append(m.premoves, san)
			m.premovePairs = append(m.premovePairs, [2]chess.Square{mv.S1(), mv.S2()})
			m.premoveList.SetMoves(m.premoves)
			m.board.SetPremoves(m.premovePairs)
			return true
		}
	}
	return false
}

// TryExecutePremove fires the first queued premove if it is now the player's turn
// and the move is legal in the actual position. Clears the queue on illegal move.
// Returns a Cmd that submits the move to the server, or nil.
func (m *GameModel) TryExecutePremove() tea.Cmd {
	if len(m.premoves) == 0 || m.gameOver {
		return nil
	}
	if m.chess.Position().Turn() != m.myColor {
		return nil
	}
	san := m.premoves[0]
	pos := m.chess.Position()
	notation := chess.AlgebraicNotation{}
	for _, mv := range m.chess.ValidMoves() {
		if notation.Encode(pos, mv) == san {
			m.premoves = m.premoves[1:]
			m.premovePairs = m.premovePairs[1:]
			m.premoveList.SetMoves(m.premoves)
			m.board.SetPremoves(m.premovePairs)
			return m.sendMoveStr(san)
		}
	}
	m.clearPremoves()
	return nil
}
