package client

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var highlightRowBg = lipgloss.Color("#3C3C3C")

var highlightRowStyle = lipgloss.NewStyle().Background(highlightRowBg)

// MoveList renders the move history for a chess game as a scrollable panel.
// Moves are displayed in pairs: "1. e4   e5", "2. Nf3  Nc6", etc.
// The current move is highlighted with a lipgloss border/background.
// The list auto-scrolls to keep the current move visible.
type MoveList struct {
	moves        []string // SAN strings in order (e.g. ["e4","e5","Nf3","Nc6"])
	currentIdx   int      // index of the current move (0-based), -1 = none
	height       int      // visible height in lines
	scrollOffset int      // first visible row index
}

// NewMoveList creates an empty move list with the given visible height.
func NewMoveList(height int) *MoveList {
	return &MoveList{
		height:     height,
		currentIdx: -1,
	}
}

// SetMoves replaces the full move list and sets the current move index.
// currentIdx is 0-based index into the moves slice (len-1 = latest move).
func (m *MoveList) SetMoves(moves []string, currentIdx int) {
	m.moves = moves
	m.currentIdx = currentIdx
	m.adjustScroll()
}

// adjustScroll updates scrollOffset so that the current row stays visible.
func (m *MoveList) adjustScroll() {
	if m.currentIdx < 0 || len(m.moves) == 0 {
		return
	}
	currentRow := m.currentIdx / 2
	if currentRow < m.scrollOffset {
		m.scrollOffset = currentRow
	} else if currentRow >= m.scrollOffset+m.height {
		m.scrollOffset = currentRow - m.height + 1
	}
}

// View renders the move list as a string.
// Format: "N. <white_move>  <black_move>" per row, or "N. <white_move>" for an incomplete pair.
// The row containing currentIdx is highlighted with a distinct background.
// Only m.height rows are shown; auto-scroll ensures current row is visible.
func (m *MoveList) View() string {
	if len(m.moves) == 0 {
		return strings.Repeat("\n", m.height)
	}
	totalRows := (len(m.moves) + 1) / 2
	var sb strings.Builder
	rendered := 0
	for row := m.scrollOffset; row < totalRows && rendered < m.height; row++ {
		whiteIdx := row * 2
		moveNum := row + 1
		white := m.moves[whiteIdx]
		black := ""
		if whiteIdx+1 < len(m.moves) {
			black = m.moves[whiteIdx+1]
		}
		line := fmt.Sprintf("%-3s %-7s %-7s", fmt.Sprintf("%d.", moveNum), white, black)
		isHighlighted := m.currentIdx >= 0 && m.currentIdx/2 == row
		if isHighlighted {
			sb.WriteString(highlightRowStyle.Render(line))
		} else {
			sb.WriteString(line)
		}
		sb.WriteString("\n")
		rendered++
	}
	return sb.String()
}
