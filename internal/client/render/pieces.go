package render

import (
	"embed"
	"image"
	"image/color"
	_ "image/png"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/notnil/chess"
)

//go:embed assets/*.png
var assetFS embed.FS

var pieceImages map[string]image.Image

var (
	renderCache   map[cellCacheKey][]string
	renderCacheMu sync.RWMutex
)

type cellCacheKey struct {
	piece chess.Piece
	bgHex string
	cols  int
	rows  int
}

var pieceTypeLetter = map[chess.PieceType]string{
	chess.King:   "k",
	chess.Queen:  "q",
	chess.Rook:   "r",
	chess.Bishop: "b",
	chess.Knight: "n",
	chess.Pawn:   "p",
}

var pieceColorLetter = map[chess.Color]string{
	chess.White: "l",
	chess.Black: "d",
}

func init() {
	pieceImages = make(map[string]image.Image)
	renderCache = make(map[cellCacheKey][]string)
	_ = fs.WalkDir(assetFS, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := d.Name()
		if len(base) < 9 {
			return nil
		}
		key := base[6:8]
		f, err := assetFS.Open(path)
		if err != nil {
			return nil
		}
		defer func() { _ = f.Close() }()
		img, _, err := image.Decode(f)
		if err != nil {
			return nil
		}
		pieceImages[key] = img
		return nil
	})
}

func hexToRGBA(hex string) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return color.RGBA{A: 255}
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

// ClearRenderCache discards all cached cell renders (call when cell size changes).
func ClearRenderCache() {
	renderCacheMu.Lock()
	renderCache = make(map[cellCacheKey][]string)
	renderCacheMu.Unlock()
	if kittyEnabled {
		clearKittyCache(os.Stdout)
	}
}

// RenderCell returns a slice of `rows` ANSI-colored strings for one board cell.
// p is the piece on the cell (chess.NoPiece for empty), bgHex is the square background
// color as a "#RRGGBB" string, cols and rows are the terminal cell dimensions.
func RenderCell(p chess.Piece, bgHex string, cols, rows int) []string {
	cacheKey := cellCacheKey{piece: p, bgHex: bgHex, cols: cols, rows: rows}
	renderCacheMu.RLock()
	if cached, ok := renderCache[cacheKey]; ok {
		renderCacheMu.RUnlock()
		return cached
	}
	renderCacheMu.RUnlock()

	var img image.Image
	if p == chess.NoPiece {
		img = image.NewNRGBA(image.Rect(0, 0, 1, 1))
	} else {
		pieceKey := pieceTypeLetter[p.Type()] + pieceColorLetter[p.Color()]
		img = pieceImages[pieceKey]
	}
	if img == nil {
		img = image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}

	bg := hexToRGBA(bgHex)
	// (rows+1)*2 pixel height accounts for ansimage's rendering loop offset:
	// the first row slot is never filled, so we request one extra terminal row.
	ai, err := ansimage.NewScaledFromImage(img, (rows+1)*2, cols, bg, ansimage.ScaleModeFill, ansimage.NoDithering)
	var lines []string
	if err == nil {
		rendered := ai.Render()
		allLines := strings.Split(rendered, "\n")
		// ansimage's NoDithering join collapses the never-set rows[0]="", so allLines[0] is the top
		// content row. Take exactly the first `rows` lines; the trailing "" is excluded by the bound.
		if len(allLines) >= rows {
			lines = append(lines, allLines[0:rows]...)
		}
	}
	for len(lines) < rows {
		lines = append(lines, strings.Repeat(" ", cols))
	}
	lines = lines[:rows]

	renderCacheMu.Lock()
	renderCache[cacheKey] = lines
	renderCacheMu.Unlock()
	return lines
}
