package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/client"
)

// Usage: shellmate [server-addr]
// server-addr defaults to "localhost:8080" if not provided.
func main() {
	serverAddr := "localhost:8080"
	if len(os.Args) > 1 {
		serverAddr = os.Args[1]
	}
	model := client.NewModel(serverAddr)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
