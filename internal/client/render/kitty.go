package render

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/notnil/chess"
)

// kittyEnabled is set once at startup by DetectKitty.
var kittyEnabled bool

const pieceSize = 60 // native PNG resolution

// DetectKitty checks whether the current terminal supports the Kitty graphics
// protocol. Must be called once before any rendering, before starting Bubbletea.
func DetectKitty() {
	term := os.Getenv("TERM")
	windowID := os.Getenv("KITTY_WINDOW_ID")
	kittyEnabled = term == "xterm-kitty" || windowID != ""
}

func composeBoardSquareBg(b *Board, sq chess.Square, fileIdx, rankIdx int) string {
	isLight := (fileIdx+rankIdx)%2 != 0
	isSelected := b.hasSelected && sq == b.selectedSquare
	isHighlighted := b.hasLastMove && (sq == b.lastMoveFrom || sq == b.lastMoveTo)
	switch {
	case isSelected && isLight:
		return string(selectedLightBg)
	case isSelected && !isLight:
		return string(selectedDarkBg)
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

// composeBoard renders all 64 squares into a single RGBA image.
// Accounts for board flip and all highlight states.
func composeBoard(b *Board) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 8*pieceSize, 8*pieceSize))
	pos := b.position
	if pos == nil {
		pos = chess.NewGame().Position()
	}
	bd := pos.Board()
	rankOrder := [8]int{7, 6, 5, 4, 3, 2, 1, 0}
	fileOrder := [8]int{0, 1, 2, 3, 4, 5, 6, 7}
	if b.flipped {
		rankOrder = [8]int{0, 1, 2, 3, 4, 5, 6, 7}
		fileOrder = [8]int{7, 6, 5, 4, 3, 2, 1, 0}
	}
	for screenRow, rankIdx := range rankOrder {
		for screenCol, fileIdx := range fileOrder {
			sq := chess.Square(rankIdx*8 + fileIdx)
			bgColor := hexToRGBA(composeBoardSquareBg(b, sq, fileIdx, rankIdx))
			cellRect := image.Rect(
				screenCol*pieceSize, screenRow*pieceSize,
				(screenCol+1)*pieceSize, (screenRow+1)*pieceSize,
			)
			draw.Draw(img, cellRect, image.NewUniform(bgColor), image.Point{}, draw.Src)
			p := bd.Piece(sq)
			if p != chess.NoPiece {
				key := pieceTypeLetter[p.Type()] + pieceColorLetter[p.Color()]
				if pieceImg := pieceImages[key]; pieceImg != nil {
					draw.Draw(img, cellRect, pieceImg, image.Point{}, draw.Over)
				}
			}
		}
	}
	return img
}

const kittyChunkSize = 4096

// buildKittyUpload writes Kitty APC sequences to w that upload img as image id.
// Uses raw RGBA format (f=32) with unicode placeholder mode (U=1).
func buildKittyUpload(img *image.RGBA, id uint8, w io.Writer) {
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	payload := base64.StdEncoding.EncodeToString(img.Pix)
	total := len(payload)
	for i := 0; i < total; i += kittyChunkSize {
		end := i + kittyChunkSize
		if end > total {
			end = total
		}
		chunk := payload[i:end]
		final := end == total
		moreFlag := "1"
		if final {
			moreFlag = "0"
		}
		if i == 0 {
			fmt.Fprintf(w, "\033_Ga=T,q=2,f=32,s=%d,v=%d,U=1,i=%d,m=%s;%s\033\\",
				width, height, id, moreFlag, chunk)
		} else {
			fmt.Fprintf(w, "\033_Gm=%s;%s\033\\", moreFlag, chunk)
		}
	}
}

// kittyPlaceholder is U+10EEEE encoded as UTF-8, the Kitty unicode placeholder char.
const kittyPlaceholder = "\U0010EEEE"

// buildPlaceholderString returns the full board view string using Kitty unicode
// placeholders. Includes rank labels (left) and file labels (bottom) matching
// the ANSI layout. id is the Kitty image ID to reference.
func buildPlaceholderString(b *Board, id uint8) string {
	rankOrder := [8]int{7, 6, 5, 4, 3, 2, 1, 0}
	fileOrder := [8]int{0, 1, 2, 3, 4, 5, 6, 7}
	if b.flipped {
		rankOrder = [8]int{0, 1, 2, 3, 4, 5, 6, 7}
		fileOrder = [8]int{7, 6, 5, 4, 3, 2, 1, 0}
	}
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	cols := b.cellCols * 8
	midLine := b.cellRows / 2
	colorCode := fmt.Sprintf("\033[38;5;%dm", id)
	resetCode := "\033[39m"
	placeholderRow := strings.Repeat(kittyPlaceholder, cols)

	var sb strings.Builder
	for _, rankIdx := range rankOrder {
		rankNum := rankIdx + 1
		for line := 0; line < b.cellRows; line++ {
			if line == midLine {
				sb.WriteString(labelStyle.Render(string(rune('0'+rankNum)) + " "))
			} else {
				sb.WriteString("  ")
			}
			sb.WriteString(colorCode)
			sb.WriteString(placeholderRow)
			sb.WriteString(resetCode)
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("  ")
	for _, fileIdx := range fileOrder {
		label := string(rune('a' + fileIdx))
		leftPad := (b.cellCols - 1) / 2
		rightPad := b.cellCols - 1 - leftPad
		sb.WriteString(labelStyle.Render(strings.Repeat(" ", leftPad) + label + strings.Repeat(" ", rightPad)))
	}
	sb.WriteByte('\n')
	return sb.String()
}

type kittyCacheKey struct {
	fen         string
	from, to    chess.Square
	sel         chess.Square
	hasLastMove bool
	hasSelected bool
	flipped     bool
	cellCols    int
	cellRows    int
}

var (
	kittyCacheMu  sync.Mutex
	kittyCacheMap       = map[kittyCacheKey]string{} // key → placeholder string
	kittyActiveID uint8 = 1                          // current image ID (1 or 2)
)

func kittyBoardKey(b *Board) kittyCacheKey {
	pos := b.position
	if pos == nil {
		pos = chess.NewGame().Position()
	}
	return kittyCacheKey{
		fen:         pos.String(),
		from:        b.lastMoveFrom,
		to:          b.lastMoveTo,
		sel:         b.selectedSquare,
		hasLastMove: b.hasLastMove,
		hasSelected: b.hasSelected,
		flipped:     b.flipped,
		cellCols:    b.cellCols,
		cellRows:    b.cellRows,
	}
}

// clearKittyCache deletes active Kitty images from the terminal and wipes the cache.
// w is the writer to send delete sequences to (os.Stdout in production).
func clearKittyCache(w io.Writer) {
	kittyCacheMu.Lock()
	defer kittyCacheMu.Unlock()
	if len(kittyCacheMap) > 0 {
		fmt.Fprintf(w, "\033_Ga=d,d=I,i=1\033\\")
		fmt.Fprintf(w, "\033_Ga=d,d=I,i=2\033\\")
		kittyCacheMap = map[kittyCacheKey]string{}
	}
}

// renderBoardKitty uploads the board image to the Kitty terminal (via w) if the
// board state has changed, then returns the placeholder string for Bubbletea.
func renderBoardKitty(b *Board, w io.Writer) string {
	key := kittyBoardKey(b)
	kittyCacheMu.Lock()
	if s, ok := kittyCacheMap[key]; ok {
		kittyCacheMu.Unlock()
		return s
	}
	// Ping-pong: switch to the other ID.
	oldID := kittyActiveID
	newID := uint8(3) - oldID // 1↔2
	kittyActiveID = newID
	kittyCacheMu.Unlock()

	img := composeBoard(b)
	buildKittyUpload(img, newID, w)
	// Delete the old image to free terminal memory.
	fmt.Fprintf(w, "\033_Ga=d,d=I,i=%d\033\\", oldID)

	placeholder := buildPlaceholderString(b, newID)

	kittyCacheMu.Lock()
	kittyCacheMap[key] = placeholder
	kittyCacheMu.Unlock()
	return placeholder
}
