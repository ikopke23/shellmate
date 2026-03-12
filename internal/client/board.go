package client

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/notnil/chess"
)

var (
	lightSquareBg      = lipgloss.Color("#F0D9B5")
	darkSquareBg       = lipgloss.Color("#B58863")
	highlightLightBg   = lipgloss.Color("#CDD16E")
	highlightDarkBg    = lipgloss.Color("#AABA44")
	pieceDarkFg        = lipgloss.Color("#2D1B00")
	pieceLightFg       = lipgloss.Color("#FFFCF0")
)

var pieceSymbols = map[chess.Color]map[chess.PieceType]string{
	chess.White: {
		chess.King:   "♔",
		chess.Queen:  "♕",
		chess.Rook:   "♖",
		chess.Bishop: "♗",
		chess.Knight: "♘",
		chess.Pawn:   "♙",
	},
	chess.Black: {
		chess.King:   "♚",
		chess.Queen:  "♛",
		chess.Rook:   "♜",
		chess.Bishop: "♝",
		chess.Knight: "♞",
		chess.Pawn:   "♟",
	},
}

// Board renders a chess board using lipgloss.
// It holds display state: the current chess.Position, which squares are highlighted,
// and whether the board is flipped (black's perspective).
type Board struct {
	position     *chess.Position
	lastMoveFrom chess.Square
	lastMoveTo   chess.Square
	flipped      bool
}

// NewBoard creates a board in the starting position, white at bottom.
func NewBoard(pos *chess.Position, flipped bool) *Board {
	return &Board{
		position: pos,
		flipped:  flipped,
	}
}

// SetPosition updates the board to a new position with last-move highlighting.
func (b *Board) SetPosition(pos *chess.Position, from, to chess.Square) {
	b.position = pos
	b.lastMoveFrom = from
	b.lastMoveTo = to
}

// View returns the rendered board as a string.
func (b *Board) View() string {
	var sb strings.Builder
	pos := b.position
	if pos == nil {
		pos = chess.NewGame().Position()
	}
	board := pos.Board()

	// ranks to render: top to bottom
	// normal: rank 7 (8th) down to rank 0 (1st)
	// flipped: rank 0 (1st) up to rank 7 (8th)
	rankOrder := [8]int{7, 6, 5, 4, 3, 2, 1, 0}
	fileOrder := [8]int{0, 1, 2, 3, 4, 5, 6, 7}
	if b.flipped {
		rankOrder = [8]int{0, 1, 2, 3, 4, 5, 6, 7}
		fileOrder = [8]int{7, 6, 5, 4, 3, 2, 1, 0}
	}

	for _, rankIdx := range rankOrder {
		rankNum := rankIdx + 1
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Render(
			strings.Repeat(" ", 0)+string(rune('0'+rankNum))+" ",
		))
		for _, fileIdx := range fileOrder {
			sq := chess.Square(rankIdx*8 + fileIdx)
			isLight := (fileIdx+rankIdx)%2 != 0
			isHighlighted := sq == b.lastMoveFrom || sq == b.lastMoveTo

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

			var fg lipgloss.Color
			if isLight {
				fg = pieceDarkFg
			} else {
				fg = pieceLightFg
			}

			piece := board.Piece(sq)
			var symbol string
			if piece == chess.NoPiece {
				symbol = " "
			} else {
				symbol = pieceSymbols[piece.Color()][piece.Type()]
			}

			cell := lipgloss.NewStyle().
				Background(bg).
				Foreground(fg).
				Render(" " + symbol + " ")
			sb.WriteString(cell)
		}
		sb.WriteString("\n")
	}

	// file labels
	sb.WriteString("   ")
	for _, fileIdx := range fileOrder {
		label := string(rune('a' + fileIdx))
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Render(" " + label + " "))
	}
	sb.WriteString("\n")

	return sb.String()
}
