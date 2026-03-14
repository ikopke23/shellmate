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

// DetectKitty checks whether the current terminal supports the Kitty graphics
// protocol. Must be called once before any rendering, before starting Bubbletea.
func DetectKitty() {
	term := os.Getenv("TERM")
	windowID := os.Getenv("KITTY_WINDOW_ID")
	kittyEnabled = term == "xterm-kitty" || windowID != ""
}

// Silence unused import errors until later tasks add the functions.
var (
	_ = base64.StdEncoding
	_ = fmt.Sprintf
	_ image.Image
	_ color.RGBA
	_ draw.Op
	_ io.Writer
	_ strings.Builder
	_ sync.Mutex
	_ lipgloss.Style
	_ chess.Square
)
