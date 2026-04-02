package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// mascotLines is an ASCII-art representation of the claw-code-go mascot —
// a chubby pixel-art cat inspired by the project logo.
// Designed to match the squat, wide-bodied blue cat in assets/claw-code-go.png.
var mascotLines = []string{
	`  /\              /\  `,
	` /  \____________/  \ `,
	`/                    \`,
	`|   ████       ████   |`,
	`|   ████       ████   |`,
	`|                     |`,
	`|      ___  ___       |`,
	`|      \  \/  /       |`,
	`|       |    |        |`,
	`|      /      \       |`,
	`|     | ||  || |      |`,
	`\     |__|  |__|      /`,
	` \____________________/`,
	`  ||   ||  ||   ||    `,
}

// RenderLogo returns a styled splash block: ASCII mascot + app name + tagline.
// It is injected into the viewport on startup so it scrolls away naturally as
// the conversation grows.
func RenderLogo(version string) string {
	bodyColor := lipgloss.Color("33")   // cornflower blue — matches logo body
	dimColor := lipgloss.Color("240")   // muted grey

	catStyle := lipgloss.NewStyle().Foreground(bodyColor)
	nameStyle := lipgloss.NewStyle().Foreground(bodyColor).Bold(true)
	verStyle := lipgloss.NewStyle().Foreground(dimColor)
	tagStyle := lipgloss.NewStyle().Foreground(dimColor).Italic(true)
	divStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	cat := catStyle.Render(strings.Join(mascotLines, "\n"))
	name := nameStyle.Render("claw-code-go")
	ver := verStyle.Render(" v" + version)
	tag := tagStyle.Render("A Go port of Claude Code")
	div := divStyle.Render(strings.Repeat("─", 22))

	return fmt.Sprintf("%s\n\n  %s%s\n  %s\n  %s\n\n", cat, name, ver, tag, div)
}
