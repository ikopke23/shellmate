package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	highlightMoveStyle = lipgloss.NewStyle().Background(lipgloss.Color("#3C3C3C"))
	dimMoveStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
)

// MoveList renders the move history for a chess game as a scrollable panel.
// Moves are displayed in pairs: "1. e4   e5", "2. Nf3  Nc6", etc.
// The current move is highlighted with a lipgloss border/background.
// The list auto-scrolls to keep the current move visible.
type MoveList struct {
	moves        []string // SAN strings in order (e.g. ["e4","e5","Nf3","Nc6"])
	currentIdx   int      // index of the current move (0-based), -1 = none
	height       int      // visible height in lines
	scrollOffset int      // first visible row index
	branchPoint  int      // first move index that is a branch move; -1 = no branch
}

// NewMoveList creates an empty move list with the given visible height.
func NewMoveList(height int) *MoveList {
	if height <= 0 {
		height = 1
	}
	return &MoveList{
		height:      height,
		currentIdx:  -1,
		branchPoint: -1,
	}
}

// SetBranchPoint marks the first branch move index. Moves before it render dimmed.
// Pass -1 to clear branch mode styling.
func (m *MoveList) SetBranchPoint(idx int) {
	m.branchPoint = idx
}

// ClickMoveIdx converts a visible row click to a 0-based move index.
// row is 0-based relative to the visible area (i.e. after scrollOffset).
// leftSide true = white move column; false = black move column.
// Returns -1 if the row or column is out of range.
func (m *MoveList) ClickMoveIdx(row int, leftSide bool) int {
	realRow := m.scrollOffset + row
	totalRows := (len(m.moves) + 1) / 2
	if realRow < 0 || realRow >= totalRows {
		return -1
	}
	if leftSide {
		idx := realRow * 2
		if idx >= len(m.moves) {
			return -1
		}
		return idx
	}
	idx := realRow*2 + 1
	if idx >= len(m.moves) {
		return -1
	}
	return idx
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
		numStr := fmt.Sprintf("%-3s ", fmt.Sprintf("%d.", moveNum))
		whiteStr := fmt.Sprintf("%-7s", white)
		blackStr := fmt.Sprintf(" %-7s", black)
		renderMove := func(idx int, s string) string {
			if m.branchPoint >= 0 && idx < m.branchPoint {
				return dimMoveStyle.Render(s)
			}
			if m.currentIdx == idx {
				return highlightMoveStyle.Render(s)
			}
			return s
		}
		sb.WriteString(numStr)
		sb.WriteString(renderMove(whiteIdx, whiteStr))
		sb.WriteString(renderMove(whiteIdx+1, blackStr))
		sb.WriteString("\n")
		rendered++
	}
	return sb.String()
}
