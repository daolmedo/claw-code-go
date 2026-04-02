package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// mascotLines is the ASCII/block-art representation of the claw-code-go mascot.
// Faithfully mirrors the pixel-art logo in assets/claw-code-go.png:
//
//	• Two small square ears at the top corners
//	• Wide, squat body (wider than tall)
//	• Two tall narrow dark rectangular eyes, symmetrically placed
//	• Central muzzle (lighter area) with a small nose, whisker dots,
//	  and two prominent buck teeth
//	• Four short stubby feet at the bottom
var mascotLines = []string{
	`  ▄▄▄                       ▄▄▄  `,
	` ▐█ █▌                     ▐█ █▌ `,
	` ▐███████████████████████████████▌`,
	` █    ▄███▄           ▄███▄    █ `,
	` █    █████           █████    █ `,
	` █    █████           █████    █ `,
	` █    ▀▀▀▀▀           ▀▀▀▀▀    █ `,
	` █                             █ `,
	` █    ──  ▐▄▄▄▄▄▄▄▌  ──       █ `,
	` █        ▐ · ▄▄ · ▌          █ `,
	` █        ▌  ██ ██  ▐         █ `,
	` ▀███████████████████████████▀  `,
	`   ▀▀▀  ▀▀▀▀    ▀▀▀▀  ▀▀▀      `,
}

// RenderLogo returns a styled splash block: ASCII mascot + app name + tagline.
// Injected into the viewport on startup; scrolls away naturally as the
// conversation grows.
func RenderLogo(version string) string {
	bodyColor := lipgloss.Color("33")  // cornflower blue — matches logo body
	dimColor  := lipgloss.Color("240") // muted grey for subtitle
	divColor  := lipgloss.Color("238") // very subtle divider

	catStyle  := lipgloss.NewStyle().Foreground(bodyColor)
	nameStyle := lipgloss.NewStyle().Foreground(bodyColor).Bold(true)
	verStyle  := lipgloss.NewStyle().Foreground(dimColor)
	tagStyle  := lipgloss.NewStyle().Foreground(dimColor).Italic(true)
	divStyle  := lipgloss.NewStyle().Foreground(divColor)

	cat  := catStyle.Render(strings.Join(mascotLines, "\n"))
	name := nameStyle.Render("claw-code-go")
	ver  := verStyle.Render(" v" + version)
	tag  := tagStyle.Render("A Go port of Claude Code")
	div  := divStyle.Render(strings.Repeat("─", 24))

	return fmt.Sprintf("%s\n\n  %s%s\n  %s\n  %s\n\n", cat, name, ver, tag, div)
}
