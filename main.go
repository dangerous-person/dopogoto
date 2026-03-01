package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dangerous-person/dopogoto/internal/ui"
)

var version = "0.1.7"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("dopogoto %s\n", version)
			return
		case "--help", "-h":
			fmt.Println("Dopo Goto — terminal music and video player")
			fmt.Println("https://github.com/dangerous-person/dopogoto")
			return
		}
	}

	app := ui.NewApp(version)

	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Give the app access to p.Send() for async player messages
	app.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
