package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PremoveList renders the queued premoves panel.
type PremoveList struct {
	moves []string
}

// NewPremoveList returns an empty premove list.
func NewPremoveList() *PremoveList { return &PremoveList{} }

// SetMoves replaces the displayed premoves.
func (pl *PremoveList) SetMoves(moves []string) {
	pl.moves = moves
}

// View renders the premove panel. Returns empty string when no premoves are queued.
func (pl *PremoveList) View() string {
	if len(pl.moves) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("Premoves") + "\n")
	for i, san := range pl.moves {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, san))
	}
	return sb.String()
}
