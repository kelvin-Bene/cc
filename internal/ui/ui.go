package ui

import (
	"fmt"
	"strings"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Cyan    = "\033[36m"
	BrCyan  = "\033[96m"
	Green   = "\033[32m"
	BrGreen = "\033[92m"
	Yellow  = "\033[33m"
	BrYell  = "\033[93m"
	Red     = "\033[31m"
	BrRed   = "\033[91m"
	White   = "\033[37m"
	BrWhite = "\033[97m"
	DkGray  = "\033[90m"
)

// Unicode symbols
const (
	Diamond = "◆"
	Bullet  = "▪"
	Arrow   = "▸"
	Check   = "✓"
	Cross   = "✗"
	Dot     = "·"
)

var logoLines = []struct {
	color string
	text  string
}{
	{BrCyan, "  ██████╗ ██╗  ██╗"},
	{BrCyan, " ██╔═══██╗██║ ██╔╝"},
	{Cyan, " ██║   ██║█████╔╝ "},
	{Cyan, " ██║▄▄ ██║██╔═██╗ "},
	{DkGray, " ╚██████╔╝██║  ██╗"},
	{DkGray, "  ╚══▀▀═╝ ╚═╝  ╚═╝"},
}

// Logo prints the qk ASCII art with an optional right-side subtitle.
func Logo(subtitle string) {
	fmt.Println()
	for i, l := range logoLines {
		fmt.Printf(" %s%s%s", l.color, l.text, Reset)
		if i == 1 && subtitle != "" {
			fmt.Printf("   %s%s%s%s", Bold, BrWhite, subtitle, Reset)
		}
		fmt.Println()
	}
}

// Sep prints a dim horizontal rule.
func Sep() {
	fmt.Printf("\n %s%s%s\n", DkGray, strings.Repeat("─", 40), Reset)
}

// Head prints a section header with a diamond bullet.
func Head(text string) {
	fmt.Printf("\n %s%s%s %s%s%s\n", BrCyan, Diamond, Reset, BrWhite, text, Reset)
}

// Ok prints a success line.
func Ok(text string) {
	fmt.Printf(" %s%s%s %s%s%s\n", BrGreen, Diamond, Reset, BrWhite, text, Reset)
}

// Warn prints a warning line.
func Warn(text string) {
	fmt.Printf("   %s%s %s%s\n", BrYell, Bullet, text, Reset)
}

// Err prints an error line.
func Err(text string) {
	fmt.Printf("   %s%s %s%s\n", BrRed, Cross, text, Reset)
}

// Item prints a list row with a pass/fail indicator.
func Item(label string, ok bool) {
	mark := fmt.Sprintf("%s%s%s", BrGreen, Check, Reset)
	if !ok {
		mark = fmt.Sprintf("%s%s%s", BrRed, Cross, Reset)
	}
	fmt.Printf("   %s%s%s %-26s %s\n", DkGray, Bullet, Reset, label, mark)
}

// Prompt prints a styled prompt. The caller still reads stdin.
func Prompt(label, defaultVal string) {
	fmt.Printf("\n %s%s%s %s%s%s", BrCyan, Diamond, Reset, BrWhite, label, Reset)
	if defaultVal != "" {
		fmt.Printf(" %s[%s]%s", DkGray, defaultVal, Reset)
	}
	fmt.Printf("\n   %s%s%s ", BrCyan, Arrow, Reset)
}

// Inline prints a single-line prompt (no newline before arrow).
func Inline(label, defaultVal string) {
	fmt.Printf(" %s%s%s %s%s%s", BrCyan, Diamond, Reset, BrWhite, label, Reset)
	if defaultVal != "" {
		fmt.Printf(" %s[%s]%s", DkGray, defaultVal, Reset)
	}
	fmt.Printf("  %s%s%s ", BrCyan, Arrow, Reset)
}

// BoxStart prints the top border of a panel.
func BoxStart(title string, badge string) {
	pad := max(34-len(title)-len(badge), 1)
	badgeStr := ""
	if badge != "" {
		badgeStr = fmt.Sprintf(" %s%s%s%s ", BrYell, Bold, badge, Reset+DkGray)
		pad = max(34-len(title)-len(badge)-2, 1)
	}
	fmt.Printf("   %s┌─ %s%s%s%s %s%s─┐%s\n",
		DkGray, BrWhite, title, Reset, DkGray+badgeStr, DkGray, strings.Repeat("─", pad), Reset)
}

// BoxRow prints a content row inside a panel.
func BoxRow(text string) {
	vis := visLen(text)
	pad := max(35-vis, 0)
	fmt.Printf("   %s│%s  %s%s%s│%s\n", DkGray, Reset, text, strings.Repeat(" ", pad), DkGray, Reset)
}

// BoxEnd prints the bottom border of a panel.
func BoxEnd() {
	fmt.Printf("   %s└%s┘%s\n", DkGray, strings.Repeat("─", 37), Reset)
}

// Fin prints a closing status line after a separator.
func Fin(text string) {
	Sep()
	Ok(text)
	fmt.Println()
}

// visLen returns the printed width of s, ignoring ANSI escapes.
func visLen(s string) int {
	n := 0
	esc := false
	for _, r := range s {
		if r == '\033' {
			esc = true
			continue
		}
		if esc {
			if r == 'm' {
				esc = false
			}
			continue
		}
		n++
	}
	return n
}
