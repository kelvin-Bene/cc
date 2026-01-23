package window

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"github.com/bcmister/quickstart/internal/monitor"
)

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	procFindWindowW   = user32.NewProc("FindWindowW")
	procSetWindowPos  = user32.NewProc("SetWindowPos")
	procEnumWindows   = user32.NewProc("EnumWindows")
	procGetWindowTextW = user32.NewProc("GetWindowTextW")
)

const (
	SWP_NOZORDER     = 0x0004
	SWP_SHOWWINDOW   = 0x0040
	HWND_TOP         = 0
)

// Position represents a window position and size
type Position struct {
	X      int
	Y      int
	Width  int
	Height int
}

// LaunchConfig holds configuration for launching a terminal
type LaunchConfig struct {
	Title       string
	WorkingDir  string
	X           int
	Y           int
	Width       int
	Height      int
	Command     string
	PostCommand string
}

// CalculateLayout calculates window positions based on layout type
func CalculateLayout(mon *monitor.Monitor, count int, layout string) []Position {
	positions := make([]Position, count)

	switch layout {
	case "grid":
		positions = calculateGrid(mon, count)
	case "vertical":
		positions = calculateVertical(mon, count)
	case "horizontal":
		positions = calculateHorizontal(mon, count)
	case "full":
		// Single window takes full monitor
		positions = []Position{{
			X:      mon.X,
			Y:      mon.Y,
			Width:  mon.Width,
			Height: mon.Height,
		}}
	default:
		// Default to grid
		positions = calculateGrid(mon, count)
	}

	return positions
}

func calculateGrid(mon *monitor.Monitor, count int) []Position {
	positions := make([]Position, count)

	// Calculate grid dimensions
	cols := 1
	rows := 1
	for cols*rows < count {
		if cols <= rows {
			cols++
		} else {
			rows++
		}
	}

	cellWidth := mon.Width / cols
	cellHeight := mon.Height / rows

	for i := 0; i < count; i++ {
		row := i / cols
		col := i % cols

		positions[i] = Position{
			X:      mon.X + (col * cellWidth),
			Y:      mon.Y + (row * cellHeight),
			Width:  cellWidth,
			Height: cellHeight,
		}
	}

	return positions
}

func calculateVertical(mon *monitor.Monitor, count int) []Position {
	positions := make([]Position, count)
	cellWidth := mon.Width / count

	for i := 0; i < count; i++ {
		positions[i] = Position{
			X:      mon.X + (i * cellWidth),
			Y:      mon.Y,
			Width:  cellWidth,
			Height: mon.Height,
		}
	}

	return positions
}

func calculateHorizontal(mon *monitor.Monitor, count int) []Position {
	positions := make([]Position, count)
	cellHeight := mon.Height / count

	for i := 0; i < count; i++ {
		positions[i] = Position{
			X:      mon.X,
			Y:      mon.Y + (i * cellHeight),
			Width:  mon.Width,
			Height: cellHeight,
		}
	}

	return positions
}

// LaunchTerminal launches a Windows Terminal window
func LaunchTerminal(cfg LaunchConfig) error {
	// Build the command that will run in the terminal
	// This runs the picker script which then runs the post-command
	terminalCmd := fmt.Sprintf(
		"powershell -ExecutionPolicy Bypass -File \"%s\" -ProjectsDir \"%s\" -PostCommand \"%s\"",
		cfg.Command,
		cfg.WorkingDir,
		cfg.PostCommand,
	)

	// Launch Windows Terminal with specific title and working directory
	// We use 'start' to launch it detached
	args := []string{
		"/c", "start", "wt",
		"-w", "quickstart",
		"--title", cfg.Title,
		"-d", cfg.WorkingDir,
		"cmd", "/k", terminalCmd,
	}

	cmd := exec.Command("cmd", args...)
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to launch terminal: %w", err)
	}

	// Wait a bit for the window to appear
	time.Sleep(500 * time.Millisecond)

	// Find the window by title and position it
	hwnd, err := findWindowByTitle(cfg.Title)
	if err != nil {
		return fmt.Errorf("failed to find window: %w", err)
	}

	// Position the window
	err = setWindowPosition(hwnd, cfg.X, cfg.Y, cfg.Width, cfg.Height)
	if err != nil {
		return fmt.Errorf("failed to position window: %w", err)
	}

	return nil
}

// findWindowByTitle finds a window handle by its title
func findWindowByTitle(title string) (uintptr, error) {
	var foundHwnd uintptr

	// Try multiple times as window might take time to appear
	for attempts := 0; attempts < 10; attempts++ {
		callback := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
			var windowTitle [256]uint16
			procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&windowTitle[0])), 256)

			text := syscall.UTF16ToString(windowTitle[:])
			if text == title || containsSubstring(text, title) {
				foundHwnd = hwnd
				return 0 // Stop enumeration
			}
			return 1 // Continue enumeration
		})

		procEnumWindows.Call(callback, 0)

		if foundHwnd != 0 {
			return foundHwnd, nil
		}

		time.Sleep(200 * time.Millisecond)
	}

	return 0, fmt.Errorf("window with title '%s' not found", title)
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// setWindowPosition positions a window at the specified coordinates
func setWindowPosition(hwnd uintptr, x, y, width, height int) error {
	ret, _, err := procSetWindowPos.Call(
		hwnd,
		HWND_TOP,
		uintptr(x),
		uintptr(y),
		uintptr(width),
		uintptr(height),
		SWP_NOZORDER|SWP_SHOWWINDOW,
	)

	if ret == 0 {
		return fmt.Errorf("SetWindowPos failed: %v", err)
	}

	return nil
}
