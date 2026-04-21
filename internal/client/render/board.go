// Package render produces ANSI and Kitty-graphics renderings of the chess
// board, pieces, and move list for the shellmate client.
package render

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/notnil/chess"
)

var (
	lightSquareBg    = lipgloss.Color("#F0D9B5")
	darkSquareBg     = lipgloss.Color("#B58863")
	highlightLightBg = lipgloss.Color("#CDD16E")
	highlightDarkBg  = lipgloss.Color("#AABA44")
	selectedLightBg  = lipgloss.Color("#F6F669")
	selectedDarkBg   = lipgloss.Color("#DAD414")
	checkLightBg     = lipgloss.Color("#FF6060")
	checkDarkBg      = lipgloss.Color("#CC2020")
)

// Board renders a chess board using lipgloss.
// It holds display state: the current chess.Position, which squares are highlighted,
// and whether the board is flipped (black's perspective).
type Board struct {
	position       *chess.Position
	lastMoveFrom   chess.Square
	lastMoveTo     chess.Square
	hasLastMove    bool
	flipped        bool
	selectedSquare chess.Square
	hasSelected    bool
	checkSquare    chess.Square
	hasCheck       bool
	cellCols       int
	cellRows       int
}

// NewBoard creates a board in the starting position, white at bottom.
func NewBoard(pos *chess.Position, flipped bool) *Board {
	return &Board{
		position:    pos,
		flipped:     flipped,
		hasLastMove: false,
		cellCols:    6,
		cellRows:    3,
	}
}

// CellCols returns the current cell width in terminal columns.
func (b *Board) CellCols() int { return b.cellCols }

// CellRows returns the current cell height in terminal rows.
func (b *Board) CellRows() int { return b.cellRows }

// SetCellSize updates the cell dimensions and clears the render cache.
func (b *Board) SetCellSize(cols, rows int) {
	b.cellCols = cols
	b.cellRows = rows
	ClearRenderCache()
}

// SetPosition updates the board to a new position with last-move highlighting.
func (b *Board) SetPosition(pos *chess.Position, from, to chess.Square) {
	b.position = pos
	b.lastMoveFrom = from
	b.lastMoveTo = to
	b.hasLastMove = true
}

// ClearHighlight removes the last-move highlighting.
func (b *Board) ClearHighlight() {
	b.hasLastMove = false
}

// SetSelected marks a square as selected.
func (b *Board) SetSelected(sq chess.Square) {
	b.selectedSquare = sq
	b.hasSelected = true
}

// ClearSelected removes the selected-square highlight.
func (b *Board) ClearSelected() {
	b.hasSelected = false
}

// SetCheck marks a square as the king-in-check square.
func (b *Board) SetCheck(sq chess.Square) {
	b.checkSquare = sq
	b.hasCheck = true
}

// ClearCheck removes the king-in-check highlight.
func (b *Board) ClearCheck() {
	b.hasCheck = false
}

// SetFlipped sets whether the board is shown from black's perspective.
func (b *Board) SetFlipped(flipped bool) {
	b.flipped = flipped
}

// Flipped returns whether the board is shown from black's perspective.
func (b *Board) Flipped() bool {
	return b.flipped
}

func (b *Board) squareBgHex(sq chess.Square, fileIdx, rankIdx int) string {
	isLight := (fileIdx+rankIdx)%2 != 0
	isSelected := b.hasSelected && sq == b.selectedSquare
	isHighlighted := b.hasLastMove && (sq == b.lastMoveFrom || sq == b.lastMoveTo)
	isCheck := b.hasCheck && sq == b.checkSquare
	switch {
	case isSelected && isLight:
		return string(selectedLightBg)
	case isSelected && !isLight:
		return string(selectedDarkBg)
	case isCheck && isLight:
		return string(checkLightBg)
	case isCheck && !isLight:
		return string(checkDarkBg)
	case isHighlighted && isLight:
		return string(highlightLightBg)
	case isHighlighted && !isLight:
		return string(highlightDarkBg)
	case isLight:
		return string(lightSquareBg)
	default:
		return string(darkSquareBg)
	}
}

// View returns the rendered board as a string.
func (b *Board) View() string {
	if kittyEnabled {
		return renderBoardKitty(b, os.Stdout)
	}
	var sb strings.Builder
	pos := b.position
	if pos == nil {
		pos = chess.NewGame().Position()
	}
	board := pos.Board()
	rankOrder := [8]int{7, 6, 5, 4, 3, 2, 1, 0}
	fileOrder := [8]int{0, 1, 2, 3, 4, 5, 6, 7}
	if b.flipped {
		rankOrder = [8]int{0, 1, 2, 3, 4, 5, 6, 7}
		fileOrder = [8]int{7, 6, 5, 4, 3, 2, 1, 0}
	}
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	for _, rankIdx := range rankOrder {
		rankNum := rankIdx + 1
		type cellInfo struct {
			bgHex string
			lines []string
		}
		cells := make([]cellInfo, 8)
		for i, fileIdx := range fileOrder {
			sq := chess.Square(rankIdx*8 + fileIdx)
			bgHex := b.squareBgHex(sq, fileIdx, rankIdx)
			p := board.Piece(sq)
			cells[i] = cellInfo{bgHex: bgHex, lines: Cell(p, bgHex, b.cellCols, b.cellRows)}
		}
		midLine := b.cellRows / 2
		for line := 0; line < b.cellRows; line++ {
			if line == midLine {
				sb.WriteString(labelStyle.Render(string(rune('0'+rankNum)) + " "))
			} else {
				sb.WriteString("  ")
			}
			for _, c := range cells {
				sb.WriteString(c.lines[line])
			}
			sb.WriteString("\n")
		}
	}
	b.renderFileLabels(&sb, fileOrder, labelStyle)
	return sb.String()
}

// renderFileLabels appends the bottom file-label row for the current flip state.
func (b *Board) renderFileLabels(sb *strings.Builder, fileOrder [8]int, labelStyle lipgloss.Style) {
	sb.WriteString("  ")
	for _, fileIdx := range fileOrder {
		label := string(rune('a' + fileIdx))
		leftPad := (b.cellCols - 1) / 2
		rightPad := b.cellCols - 1 - leftPad
		sb.WriteString(labelStyle.Render(strings.Repeat(" ", leftPad) + label + strings.Repeat(" ", rightPad)))
	}
	sb.WriteString("\n")
}
