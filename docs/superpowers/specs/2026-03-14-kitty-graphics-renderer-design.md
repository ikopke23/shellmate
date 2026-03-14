# Kitty Graphics Renderer Design

**Date:** 2026-03-14
**Status:** Approved

## Overview

Add a Kitty terminal graphics protocol renderer for the chess board that delivers pixel-perfect piece images. The current half-block ansimage renderer remains fully intact as the fallback for non-Kitty terminals.

## Goals

- Render chess pieces at native PNG quality in Kitty terminals
- Zero impact on non-Kitty terminals (existing ANSI path unchanged)
- No added latency to WebSocket communication (render path is independent)
- Detection happens once at startup, fixed for the session

## Architecture

All changes are contained in `internal/client/render/`. The Kitty and ANSI paths share only the `pieceImages` map loaded in `pieces.go`.

### New File: `internal/client/render/kitty.go`

**`DetectKitty() bool`**
- Checks `$TERM == "xterm-kitty"` and/or `$KITTY_WINDOW_ID` is non-empty
- Called once at program startup, result stored in package-level `kittyEnabled bool`
- No I/O; env var check only

**`ComposeBoard(b *Board) *image.RGBA`**
- Composites all 64 squares into a single `480×480` RGBA image (8×60px per cell, native PNG resolution)
- Draws square background colors first (same hex values as ANSI path)
- Composites piece PNGs from `pieceImages` over each square using `draw.Over`
- Applies highlight and selection colors the same way as ANSI path

**`renderBoardKitty(b *Board) string`**
- Checks `kittyCache` by board state hash; returns cached placeholder string on hit
- On miss: calls `ComposeBoard`, uploads image to terminal, builds placeholder string, caches
- Writes Kitty APC sequences directly to `os.Stdout` before returning the string to Bubbletea

### Kitty Protocol: Image Upload

Raw RGBA bytes (no PNG compression) are base64-encoded and sent in ≤4096-byte chunks:

```
ESC_G a=T,q=2,f=32,s=480,v=480,U=1,i=<id>,m=1;<chunk> ESC\
ESC_G a=m=0;<finalChunk>                              ESC\
```

- `a=T`: transmit action
- `f=32`: RGBA pixel format
- `U=1`: unicode placeholder mode
- `q=2`: suppress terminal ACK (no response to parse)
- `i=<id>`: image ID, 1–255 (ping-pong between 2 IDs)
- `m=1/0`: more chunks / final chunk

### Kitty Protocol: Unicode Placeholder String

After upload, each row of the board is rendered as:

```
ESC[38;5;<id>m + (8×cellCols × U+10EEEE) + ESC[39m + \n
```

Total: `8×cellRows` rows of `8×cellCols` placeholder characters. Kitty automatically maps each character cell to the corresponding pixel region of the uploaded image.

### Caching & Image Lifecycle

**Cache key:**
```
"<fen>|<fromSq>-<toSq>|<selSq>|<hasLastMove><hasSelected>|<cellCols>x<cellRows>"
```

**Cache value:** `(imageID uint8, placeholderString string)`

**Ping-pong IDs:** Two image IDs (1 and 2) alternate. When a new board state is rendered:
1. Upload new image to the inactive ID
2. Return placeholder string referencing the new ID
3. Delete the old ID: `ESC_G a=d,d=I,i=<oldID> ESC\`

**On resize:** `ClearRenderCache()` deletes both active Kitty images and clears the kitty cache map.

### Performance

- Compositing 64×(60×60) pixels: ~3.6M pixel ops, <2ms
- Base64 encoding 480×480×4 = 921KB → ~1.2MB output, written once per board state change
- Cache hit path: returns pre-built string in microseconds
- WebSocket goroutine is independent; no coupling to render path

## Modified Files

| File | Change |
|------|--------|
| `internal/client/render/kitty.go` | **New** — all Kitty logic |
| `internal/client/render/board.go` | Add `if kittyEnabled { return renderBoardKitty(b) }` at top of `View()` |
| `internal/client/render/pieces.go` | Extend `ClearRenderCache()` to also delete active Kitty images |
| `cmd/` entry point | Call `render.DetectKitty()` once before starting Bubbletea |

## What Does Not Change

- `RenderCell` and the ANSI half-block path
- `pieces.go` image loading and ANSI cache
- `game.go` (promotion popup, resize keys, mouse mapping)
- `movelist.go`
- Server code
