package screens

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/client/render"
	"github.com/notnil/chess"
)

// LocalMoveInput handles board click selection, SAN text entry, and promotion popups
// for both live and local (branch) play modes.
type LocalMoveInput struct {
	textInput        textinput.Model
	selectedSq       chess.Square
	hasSelected      bool
	pendingPromo     bool
	pendingPromoFrom chess.Square
	pendingPromoTo   chess.Square
	promoPopupY      int
	flipped          bool
}

func NewLocalMoveInput(flipped bool) *LocalMoveInput {
	ti := textinput.New()
	ti.Placeholder = "Type move (e.g. e4)"
	ti.Focus()
	ti.CharLimit = 10
	ti.Width = 20
	return &LocalMoveInput{textInput: ti, flipped: flipped}
}

func (li *LocalMoveInput) Init() tea.Cmd {
	return textinput.Blink
}

func (li *LocalMoveInput) SetPromoPopupY(y int) {
	li.promoPopupY = y
}

func (li *LocalMoveInput) PendingPromo() bool {
	return li.pendingPromo
}

// HandleMsg processes a tea.Msg for move input.
// Returns (san, handled, cmd). san non-empty = complete move. handled = parent should not process further.
func (li *LocalMoveInput) HandleMsg(msg tea.Msg, board *render.Board, game *chess.Game) (san string, handled bool, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if li.pendingPromo {
				san = li.handlePromoClick(msg.X, msg.Y, board, game)
				return san, true, nil
			}
			sq, ok := li.squareFromMouse(msg.X, msg.Y, board)
			if ok {
				san = li.handleSquareClick(sq, board, game)
				return san, true, nil
			}
		}
	case tea.KeyMsg:
		if li.pendingPromo {
			switch msg.String() {
			case "q", "r", "b", "n":
				san = li.promoSAN(msg.String(), game)
				li.pendingPromo = false
				return san, true, nil
			case "esc":
				li.pendingPromo = false
				return "", true, nil
			}
			return "", true, nil
		}
		if msg.String() == "enter" {
			san = li.submitSAN(game)
			return san, true, nil
		}
	}
	li.textInput, cmd = li.textInput.Update(msg)
	return "", false, cmd
}

func (li *LocalMoveInput) submitSAN(game *chess.Game) string {
	san := strings.TrimSpace(li.textInput.Value())
	if san == "" {
		return ""
	}
	pos := game.Position()
	notation := chess.AlgebraicNotation{}
	for _, mv := range game.ValidMoves() {
		if notation.Encode(pos, mv) == san {
			li.textInput.SetValue("")
			return san
		}
	}
	li.textInput.SetValue("")
	return ""
}

func (li *LocalMoveInput) handleSquareClick(sq chess.Square, board *render.Board, game *chess.Game) string {
	pos := game.Position()
	piece := pos.Board().Piece(sq)
	turn := pos.Turn()
	if !li.hasSelected {
		if piece != chess.NoPiece && piece.Color() == turn {
			li.selectedSq = sq
			li.hasSelected = true
			board.SetSelected(sq)
		}
		return ""
	}
	if sq == li.selectedSq {
		li.hasSelected = false
		board.ClearSelected()
		return ""
	}
	if piece != chess.NoPiece && piece.Color() == turn {
		li.selectedSq = sq
		board.SetSelected(sq)
		return ""
	}
	from := li.selectedSq
	li.hasSelected = false
	board.ClearSelected()
	if li.isPromotionMove(from, sq, game) {
		li.pendingPromo = true
		li.pendingPromoFrom = from
		li.pendingPromoTo = sq
		return ""
	}
	return li.mouseToSAN(from, sq, game)
}

func (li *LocalMoveInput) squareFromMouse(x, y int, board *render.Board) (chess.Square, bool) {
	cellCols := board.CellCols()
	cellRows := board.CellRows()
	if x < 2 || x > 2+8*cellCols-1 || y < 0 || y > 8*cellRows-1 {
		return 0, false
	}
	cellCol := (x - 2) / cellCols
	cellRow := y / cellRows
	if cellCol < 0 || cellCol > 7 {
		return 0, false
	}
	var rankIdx, fileIdx int
	if board.Flipped() {
		rankIdx = cellRow
		fileIdx = 7 - cellCol
	} else {
		rankIdx = 7 - cellRow
		fileIdx = cellCol
	}
	return chess.Square(rankIdx*8 + fileIdx), true
}

func (li *LocalMoveInput) mouseToSAN(from, to chess.Square, game *chess.Game) string {
	pos := game.Position()
	var bestMove *chess.Move
	for _, mv := range game.ValidMoves() {
		if mv.S1() == from && mv.S2() == to {
			if mv.Promo() == chess.Queen {
				bestMove = mv
				break
			}
			if bestMove == nil {
				bestMove = mv
			}
		}
	}
	if bestMove == nil {
		return ""
	}
	return chess.AlgebraicNotation{}.Encode(pos, bestMove)
}

func (li *LocalMoveInput) isPromotionMove(from, to chess.Square, game *chess.Game) bool {
	for _, mv := range game.ValidMoves() {
		if mv.S1() == from && mv.S2() == to && mv.Promo() != chess.NoPieceType {
			return true
		}
	}
	return false
}

func (li *LocalMoveInput) promoSAN(key string, game *chess.Game) string {
	promoMap := map[string]chess.PieceType{
		"q": chess.Queen,
		"r": chess.Rook,
		"b": chess.Bishop,
		"n": chess.Knight,
	}
	promo, ok := promoMap[key]
	if !ok {
		return ""
	}
	pos := game.Position()
	for _, mv := range game.ValidMoves() {
		if mv.S1() == li.pendingPromoFrom && mv.S2() == li.pendingPromoTo && mv.Promo() == promo {
			return chess.AlgebraicNotation{}.Encode(pos, mv)
		}
	}
	return ""
}

func (li *LocalMoveInput) handlePromoClick(x, y int, board *render.Board, game *chess.Game) string {
	cols := board.CellCols()
	rows := board.CellRows()
	pieceY := li.promoPopupY + 2
	if y < pieceY || y >= pieceY+rows {
		return ""
	}
	if x < 2 || x >= 2+4*cols {
		return ""
	}
	pieceIdx := (x - 2) / cols
	keys := []string{"q", "r", "b", "n"}
	if pieceIdx < 0 || pieceIdx >= len(keys) {
		return ""
	}
	san := li.promoSAN(keys[pieceIdx], game)
	li.pendingPromo = false
	return san
}

// View renders either the promotion popup or the SAN text input line.
// Note: promoPopupView uses gameStatusStyle and gameHelpStyle from game.go (same package).
func (li *LocalMoveInput) View(board *render.Board, game *chess.Game) string {
	if li.pendingPromo {
		return promoPopupView(board, game)
	}
	return li.textInput.View() + "\n"
}

// promoPopupView renders the four-piece promotion selection popup.
func promoPopupView(board *render.Board, game *chess.Game) string {
	var sb strings.Builder
	myColor := game.Position().Turn()
	type promoOpt struct {
		pt  chess.PieceType
		key string
	}
	opts := []promoOpt{
		{chess.Queen, "q"},
		{chess.Rook, "r"},
		{chess.Bishop, "b"},
		{chess.Knight, "n"},
	}
	bgs := []string{"#F0D9B5", "#B58863", "#F0D9B5", "#B58863"}
	cols := board.CellCols()
	rows := board.CellRows()
	sb.WriteString(gameStatusStyle.Render("Promote pawn:") + "\n")
	for line := 0; line < rows; line++ {
		sb.WriteString("  ")
		for i, opt := range opts {
			p := chess.NewPiece(opt.pt, myColor)
			lines := render.RenderCell(p, bgs[i], cols, rows)
			sb.WriteString(lines[line])
		}
		sb.WriteString("\n")
	}
	sb.WriteString("  ")
	for _, opt := range opts {
		leftPad := (cols - 1) / 2
		rightPad := cols - 1 - leftPad
		label := strings.Repeat(" ", leftPad) + opt.key + strings.Repeat(" ", rightPad)
		sb.WriteString(gameHelpStyle.Render(label))
	}
	sb.WriteString("\n")
	return sb.String()
}
