# Kitty Graphics Renderer Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Kitty terminal graphics protocol renderer that uploads the full chess board as a single PNG-quality image, falling back transparently to the existing ansimage half-block renderer on non-Kitty terminals.

**Architecture:** `DetectKitty()` is called once at startup and sets a package-level `kittyEnabled` bool. `Board.View()` gains a single branch: if enabled, call `renderBoardKitty(b, os.Stdout)` which composites all 64 squares into a 480×480 RGBA image, uploads it to the terminal via Kitty APC escape sequences, then returns a string of `U+10EEEE` placeholder characters that Bubbletea renders normally. Results are cached by board state hash; only recomposed on board change.

**Tech Stack:** Go stdlib `image`, `image/draw`, `encoding/base64`, `fmt`, `os`; `github.com/charmbracelet/lipgloss` (labels); `github.com/notnil/chess`; Kitty graphics protocol (unicode placeholder mode).

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/client/render/kitty.go` | **Create** | All Kitty logic: detection, compositing, upload, placeholder building, cache |
| `internal/client/render/kitty_test.go` | **Create** | Unit tests for all Kitty functions |
| `internal/client/render/board.go` | **Modify** | Add Kitty branch at top of `View()` |
| `internal/client/render/pieces.go` | **Modify** | Extend `ClearRenderCache()` to also wipe Kitty cache |
| `cmd/shellmate/main.go` | **Modify** | Call `render.DetectKitty()` before starting Bubbletea |

---

## Chunk 1: Detection, Compositing, Upload

### Task 1: Detection

**Files:**
- Create: `internal/client/render/kitty.go`
- Create: `internal/client/render/kitty_test.go`

- [ ] **Step 1: Create `kitty.go` with detection**

```go
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
```

- [ ] **Step 2: Write failing test**

```go
package render

import (
	"testing"
)

func TestDetectKitty_Term(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	t.Setenv("KITTY_WINDOW_ID", "")
	DetectKitty()
	if !kittyEnabled {
		t.Error("expected kittyEnabled=true with TERM=xterm-kitty")
	}
}

func TestDetectKitty_WindowID(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("KITTY_WINDOW_ID", "3")
	DetectKitty()
	if !kittyEnabled {
		t.Error("expected kittyEnabled=true with KITTY_WINDOW_ID set")
	}
}

func TestDetectKitty_None(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("KITTY_WINDOW_ID", "")
	DetectKitty()
	if kittyEnabled {
		t.Error("expected kittyEnabled=false with no Kitty env vars")
	}
}
```

- [ ] **Step 3: Run tests**

```bash
cd /home/ikopke/Github/shellmate && go test ./internal/client/render/ -run TestDetectKitty -v
```
Expected: all 3 pass.

- [ ] **Step 4: Commit**

```bash
git -C /home/ikopke/Github/shellmate add internal/client/render/kitty.go internal/client/render/kitty_test.go
git -C /home/ikopke/Github/shellmate commit -m "add Kitty terminal detection"
```

---

### Task 2: Board image compositing

The board is composited as a 480×480 RGBA image (8 squares × 60px per piece PNG). Each square gets its background color filled first, then the piece PNG composited over using `draw.Over` (preserves piece PNG transparency).

**Files:**
- Modify: `internal/client/render/kitty.go`
- Modify: `internal/client/render/kitty_test.go`

- [ ] **Step 1: Add `composeBoard` to `kitty.go`**

```go
const pieceSize = 60 // native PNG resolution

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
```

- [ ] **Step 2: Write failing tests**

```go
func TestComposeBoard_Dimensions(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	img := composeBoard(b)
	if img.Bounds().Dx() != 480 || img.Bounds().Dy() != 480 {
		t.Errorf("expected 480x480, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestComposeBoard_EmptySquareBg(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	img := composeBoard(b)
	// e4 (rank 4 idx 3, file e idx 4): isLight=(4+3)%2=1 → light square, no piece in starting pos
	// Not flipped: rankOrder[4]=3 → screenRow=4; fileOrder[4]=4 → screenCol=4
	cx := 4*pieceSize + pieceSize/2
	cy := 4*pieceSize + pieceSize/2
	got := img.RGBAAt(cx, cy)
	want := hexToRGBA(string(lightSquareBg))
	if got != want {
		t.Errorf("e4 bg: want %v, got %v", want, got)
	}
}

func TestComposeBoard_Flipped(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), true)
	img := composeBoard(b)
	if img.Bounds().Dx() != 480 || img.Bounds().Dy() != 480 {
		t.Errorf("expected 480x480, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/client/render/ -run TestComposeBoard -v
```
Expected: all 3 pass.

- [ ] **Step 4: Commit**

```bash
git -C /home/ikopke/Github/shellmate add internal/client/render/kitty.go internal/client/render/kitty_test.go
git -C /home/ikopke/Github/shellmate commit -m "add Kitty board image compositing"
```

---

### Task 3: Kitty image upload sequence

Uploads the RGBA image via APC escape sequences. The `image.RGBA.Pix` field is raw RGBA bytes (no encoding overhead), base64-encoded and split into ≤4096-char chunks.

**Files:**
- Modify: `internal/client/render/kitty.go`
- Modify: `internal/client/render/kitty_test.go`

- [ ] **Step 1: Add `buildKittyUpload` to `kitty.go`**

```go
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
```

- [ ] **Step 2: Write failing tests**

Add to `kitty_test.go`:

```go
import (
	"bytes"
	"image"
	"testing"
)

func TestBuildKittyUpload_Format(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	buildKittyUpload(img, 1, &buf)
	out := buf.String()

	checks := []string{"a=T", "f=32", "U=1", "i=1", "s=4", "v=4", "m=0"}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in upload sequence: %q", c, out[:min(len(out), 80)])
		}
	}
	if !strings.HasPrefix(out, "\033_G") {
		t.Errorf("expected APC start \\033_G, got: %q", out[:min(len(out), 10)])
	}
	if !strings.HasSuffix(out, "\033\\") {
		t.Errorf("expected APC end \\033\\\\, got: %q", out[max(0, len(out)-5):])
	}
}

func TestBuildKittyUpload_Chunking(t *testing.T) {
	// 120x120 image → 120*120*4=57600 bytes → base64 ~76800 chars → ~19 chunks
	img := image.NewRGBA(image.Rect(0, 0, 120, 120))
	var buf bytes.Buffer
	buildKittyUpload(img, 2, &buf)
	out := buf.String()
	// First chunk must have m=1 (more data follows)
	firstEnd := strings.Index(out, "\033\\")
	firstChunk := out[:firstEnd]
	if !strings.Contains(firstChunk, "m=1") {
		t.Errorf("first chunk should have m=1: %q", firstChunk[:min(len(firstChunk), 80)])
	}
	// Last chunk must have m=0
	lastStart := strings.LastIndex(out, "\033_G")
	lastChunk := out[lastStart:]
	if !strings.Contains(lastChunk, "m=0") {
		t.Errorf("last chunk should have m=0: %q", lastChunk[:min(len(lastChunk), 80)])
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/client/render/ -run TestBuildKittyUpload -v
```
Expected: both pass.

- [ ] **Step 4: Commit**

```bash
git -C /home/ikopke/Github/shellmate add internal/client/render/kitty.go internal/client/render/kitty_test.go
git -C /home/ikopke/Github/shellmate commit -m "add Kitty image upload sequence builder"
```

---

## Chunk 2: Placeholder String, Cache, Wiring

### Task 4: Placeholder string with rank/file labels

`buildPlaceholderString` returns the full view string: `8×cellRows` rows of placeholder chars with rank labels on the left (matching the ANSI layout), plus file labels at the bottom.

**Files:**
- Modify: `internal/client/render/kitty.go`
- Modify: `internal/client/render/kitty_test.go`

- [ ] **Step 1: Add `kittyPlaceholder` const and `buildPlaceholderString` to `kitty.go`**

```go
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
```

- [ ] **Step 2: Write failing tests**

```go
func TestBuildPlaceholderString_RowCount(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	b.cellCols = 6
	b.cellRows = 3
	s := buildPlaceholderString(b, 1)
	// 8 ranks × 3 lines + 1 file label line = 25 lines
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) != 25 {
		t.Errorf("expected 25 lines (8*3+1), got %d", len(lines))
	}
}

func TestBuildPlaceholderString_PlaceholderChars(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	b.cellCols = 6
	b.cellRows = 3
	s := buildPlaceholderString(b, 1)
	lines := strings.Split(s, "\n")
	// First content line (line 0): should have 8*6=48 placeholder chars
	count := strings.Count(lines[0], kittyPlaceholder)
	if count != 48 {
		t.Errorf("expected 48 placeholder chars per line, got %d", count)
	}
}

func TestBuildPlaceholderString_IDColorCode(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	b.cellCols = 6
	b.cellRows = 3
	s := buildPlaceholderString(b, 2)
	if !strings.Contains(s, "\033[38;5;2m") {
		t.Errorf("expected color code for id=2, not found in output")
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/client/render/ -run TestBuildPlaceholderString -v
```
Expected: all 3 pass.

- [ ] **Step 4: Commit**

```bash
git -C /home/ikopke/Github/shellmate add internal/client/render/kitty.go internal/client/render/kitty_test.go
git -C /home/ikopke/Github/shellmate commit -m "add Kitty placeholder string builder"
```

---

### Task 5: Cache and `renderBoardKitty`

Adds the cache (keyed by board state) and the full pipeline function. Ping-pong between image IDs 1 and 2 so only one deletion is needed per board change.

**Files:**
- Modify: `internal/client/render/kitty.go`

- [ ] **Step 1: Add cache types and `renderBoardKitty` to `kitty.go`**

```go
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
	kittyCacheMap = map[kittyCacheKey]string{} // key → placeholder string
	kittyActiveID uint8 = 1                    // current image ID (1 or 2)
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
```

- [ ] **Step 2: Write a smoke test for `renderBoardKitty`**

```go
func TestRenderBoardKitty_ReturnsSomething(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	var buf bytes.Buffer
	result := renderBoardKitty(b, &buf)
	if result == "" {
		t.Error("expected non-empty placeholder string")
	}
	if !strings.Contains(result, kittyPlaceholder) {
		t.Error("expected placeholder chars in result")
	}
	// Upload sequence should have been written to buf
	if !strings.Contains(buf.String(), "\033_G") {
		t.Error("expected Kitty APC sequence written to writer")
	}
}

func TestRenderBoardKitty_CacheHit(t *testing.T) {
	// Reset cache state for test isolation
	kittyCacheMu.Lock()
	kittyCacheMap = map[kittyCacheKey]string{}
	kittyActiveID = 1
	kittyCacheMu.Unlock()

	b := NewBoard(chess.NewGame().Position(), false)
	var buf1, buf2 bytes.Buffer
	r1 := renderBoardKitty(b, &buf1)
	r2 := renderBoardKitty(b, &buf2)
	if r1 != r2 {
		t.Error("expected identical results for same board state")
	}
	// Second call should not upload again
	if buf2.Len() != 0 {
		t.Errorf("expected no upload on cache hit, got %d bytes written", buf2.Len())
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/client/render/ -run TestRenderBoardKitty -v
```
Expected: both pass.

- [ ] **Step 4: Commit**

```bash
git -C /home/ikopke/Github/shellmate add internal/client/render/kitty.go internal/client/render/kitty_test.go
git -C /home/ikopke/Github/shellmate commit -m "add Kitty render cache and pipeline"
```

---

### Task 6: Wire into `Board.View()`, `ClearRenderCache()`, and `main.go`

**Files:**
- Modify: `internal/client/render/board.go`
- Modify: `internal/client/render/pieces.go`
- Modify: `cmd/shellmate/main.go`

- [ ] **Step 1: Add Kitty branch to `Board.View()` in `board.go`**

At the very top of `View()`, before the `var sb strings.Builder` line, add:

```go
if kittyEnabled {
    return renderBoardKitty(b, os.Stdout)
}
```

Also add `"os"` to the import block.

- [ ] **Step 2: Extend `ClearRenderCache()` in `pieces.go`**

```go
// ClearRenderCache discards all cached cell renders (call when cell size changes).
func ClearRenderCache() {
	renderCacheMu.Lock()
	renderCache = make(map[cellCacheKey][]string)
	renderCacheMu.Unlock()
	if kittyEnabled {
		clearKittyCache(os.Stdout)
	}
}
```

Also add `"os"` to the import block in `pieces.go`.

- [ ] **Step 3: Call `DetectKitty()` in `main.go`**

```go
func main() {
	serverAddr := "localhost:8080"
	if len(os.Args) > 1 {
		serverAddr = os.Args[1]
	}
	render.DetectKitty() // detect before Bubbletea takes over the terminal
	model := client.NewModel(serverAddr)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

Add `"github.com/ikopke/shellmate/internal/client/render"` to imports.

- [ ] **Step 4: Build**

```bash
go build ./... 2>&1
```
Expected: no errors.

- [ ] **Step 5: Format and vet**

```bash
go fmt ./... && go vet ./... 2>&1
```
Expected: no output.

- [ ] **Step 6: Run all tests**

```bash
go test ./internal/client/render/ -v 2>&1
```
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git -C /home/ikopke/Github/shellmate add internal/client/render/board.go internal/client/render/pieces.go cmd/shellmate/main.go
git -C /home/ikopke/Github/shellmate commit -m "wire Kitty renderer into board view and startup detection"
```
