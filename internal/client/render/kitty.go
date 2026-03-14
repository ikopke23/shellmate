package render

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
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
			isLight := (fileIdx+rankIdx)%2 != 0
			isSelected := b.hasSelected && sq == b.selectedSquare
			isHighlighted := b.hasLastMove && (sq == b.lastMoveFrom || sq == b.lastMoveTo)
			var bgHex string
			switch {
			case isSelected && isLight:
				bgHex = string(selectedLightBg)
			case isSelected && !isLight:
				bgHex = string(selectedDarkBg)
			case isHighlighted && isLight:
				bgHex = string(highlightLightBg)
			case isHighlighted && !isLight:
				bgHex = string(highlightDarkBg)
			case isLight:
				bgHex = string(lightSquareBg)
			default:
				bgHex = string(darkSquareBg)
			}
			bgColor := hexToRGBA(bgHex)
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

// Silence unused import errors until later tasks add the functions.
var (
	_ = base64.StdEncoding
	_ = fmt.Sprintf
	_ color.RGBA
	_ io.Writer
	_ strings.Builder
	_ sync.Mutex
	_ lipgloss.Style
)
