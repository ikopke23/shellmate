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
	position     *chess.Position
	lastMoveFrom chess.Square
	lastMoveTo   chess.Square
	hasLastMove  bool
	flipped      bool
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

// SetFlipped sets whether the board is shown from black's perspective.
func (b *Board) SetFlipped(flipped bool) {
	b.flipped = flipped
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
	for _, rankIdx := range rankOrder {
		rankNum := rankIdx + 1
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Render(
			string(rune('0'+rankNum)) + " ",
		))
		for _, fileIdx := range fileOrder {
			sq := chess.Square(rankIdx*8 + fileIdx)
			isLight := (fileIdx+rankIdx)%2 != 0
			isHighlighted := b.hasLastMove && (sq == b.lastMoveFrom || sq == b.lastMoveTo)
			var bg lipgloss.Color
			switch {
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
			var symbol string
			if p == chess.NoPiece {
				symbol = " "
			} else {
				symbol = pieceSymbols[p.Color()][p.Type()]
			}
			cell := lipgloss.NewStyle().
				Background(bg).
				Foreground(fg).
				Render(" " + symbol + " ")
			sb.WriteString(cell)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("   ")
	for _, fileIdx := range fileOrder {
		label := string(rune('a' + fileIdx))
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Render(" " + label + " "))
	}
	sb.WriteString("\n")
	return sb.String()
}
