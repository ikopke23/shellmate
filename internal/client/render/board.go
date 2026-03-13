package render

import (
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
	pieceDarkFg      = lipgloss.Color("#2D1B00")
	pieceLightFg     = lipgloss.Color("#FFFCF0")
)

var pieceSymbols = map[chess.Color]map[chess.PieceType]string{
	chess.White: {
		chess.King:   "\u2654",
		chess.Queen:  "\u2655",
		chess.Rook:   "\u2656",
		chess.Bishop: "\u2657",
		chess.Knight: "\u2658",
		chess.Pawn:   "\u2659",
	},
	chess.Black: {
		chess.King:   "\u265a",
		chess.Queen:  "\u265b",
		chess.Rook:   "\u265c",
		chess.Bishop: "\u265d",
		chess.Knight: "\u265e",
		chess.Pawn:   "\u265f",
	},
}

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
}

// NewBoard creates a board in the starting position, white at bottom.
func NewBoard(pos *chess.Position, flipped bool) *Board {
	return &Board{
		position:    pos,
		flipped:     flipped,
		hasLastMove: false,
	}
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

// SetFlipped sets whether the board is shown from black's perspective.
func (b *Board) SetFlipped(flipped bool) {
	b.flipped = flipped
}

// Flipped returns whether the board is shown from black's perspective.
func (b *Board) Flipped() bool {
	return b.flipped
}

// View returns the rendered board as a string.
func (b *Board) View() string {
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
			bg     lipgloss.Color
			fg     lipgloss.Color
			symbol string
		}
		cells := make([]cellInfo, 8)
		for i, fileIdx := range fileOrder {
			sq := chess.Square(rankIdx*8 + fileIdx)
			isLight := (fileIdx+rankIdx)%2 != 0
			isSelected := b.hasSelected && sq == b.selectedSquare
			isHighlighted := b.hasLastMove && (sq == b.lastMoveFrom || sq == b.lastMoveTo)
			var bg lipgloss.Color
			switch {
			case isSelected && isLight:
				bg = selectedLightBg
			case isSelected && !isLight:
				bg = selectedDarkBg
			case isHighlighted && isLight:
				bg = highlightLightBg
			case isHighlighted && !isLight:
				bg = highlightDarkBg
			case isLight:
				bg = lightSquareBg
			default:
				bg = darkSquareBg
			}
			p := board.Piece(sq)
			var fg lipgloss.Color
			if p == chess.NoPiece {
				if isLight {
					fg = pieceDarkFg
				} else {
					fg = pieceLightFg
				}
			} else if p.Color() == chess.White {
				fg = pieceDarkFg
			} else {
				fg = pieceLightFg
			}
			symbol := " "
			if p != chess.NoPiece {
				symbol = pieceSymbols[p.Color()][p.Type()]
			}
			cells[i] = cellInfo{bg: bg, fg: fg, symbol: symbol}
		}
		// Each rank renders as 3 terminal lines; piece centered on the middle line.
		// Cells are 6 chars wide x 3 lines tall for a visually square appearance.
		for line := 0; line < 3; line++ {
			if line == 1 {
				sb.WriteString(labelStyle.Render(string(rune('0'+rankNum)) + " "))
			} else {
				sb.WriteString("  ")
			}
			for _, c := range cells {
				style := lipgloss.NewStyle().Background(c.bg).Foreground(c.fg)
				if line == 1 {
					sb.WriteString(style.Render("  " + c.symbol + "   "))
				} else {
					sb.WriteString(style.Render("      "))
				}
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("  ")
	for _, fileIdx := range fileOrder {
		label := string(rune('a' + fileIdx))
		sb.WriteString(labelStyle.Render("  " + label + "   "))
	}
	sb.WriteString("\n")
	return sb.String()
}
