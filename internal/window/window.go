package window

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/bcmister/qk/internal/monitor"
)

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	procFindWindowW    = user32.NewProc("FindWindowW")
	procSetWindowPos   = user32.NewProc("SetWindowPos")
	procEnumWindows    = user32.NewProc("EnumWindows")
	procGetWindowTextW = user32.NewProc("GetWindowTextW")
)

const (
	SWP_NOZORDER  = 0x0004
	SWP_SHOWWINDOW = 0x0040
	HWND_TOP       = 0
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
	Title      string
	WorkingDir string
	X          int
	Y          int
	Width      int
	Height     int
	Command    string // command to run after project selection
}

// CalculateLayout calculates window positions based on layout type
func CalculateLayout(mon *monitor.Monitor, count int, layout string) []Position {
	switch layout {
	case "grid":
		return calculateGrid(mon, count)
	case "vertical":
		return calculateVertical(mon, count)
	case "horizontal":
		return calculateHorizontal(mon, count)
	case "full":
		return []Position{{
			X:      mon.X,
			Y:      mon.Y,
			Width:  mon.Width,
			Height: mon.Height,
		}}
	default:
		return calculateGrid(mon, count)
	}
}

func calculateGrid(mon *monitor.Monitor, count int) []Position {
	positions := make([]Position, count)

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

// LaunchTerminal launches a Windows Terminal window with an inline project picker
func LaunchTerminal(cfg LaunchConfig) error {
	// Escape single quotes in paths for PowerShell
	escapedDir := strings.ReplaceAll(cfg.WorkingDir, "'", "''")
	escapedCmd := strings.ReplaceAll(cfg.Command, "'", "''")

	// Inline PowerShell picker: list subdirs, read selection, cd and run command
	picker := fmt.Sprintf(
		"$d='%s'; $p=Get-ChildItem $d -Dir; if($p.Count -eq 0){Write-Host 'No projects found in' $d; Read-Host; exit}; "+
			"Write-Host ''; Write-Host ('Projects in ' + $d + ':') -ForegroundColor Cyan; Write-Host ''; "+
			"$i=1; $p|%%{Write-Host ('  [' + $i + '] ' + $_.Name); $i++}; Write-Host ''; "+
			"$s=Read-Host 'Pick'; "+
			"$t=$p[[int]$s-1].FullName; Set-Location $t; "+
			"Write-Host ''; Write-Host ('Opening ' + (Split-Path $t -Leaf) + '...') -ForegroundColor Green; Write-Host ''; "+
			"Invoke-Expression '%s'",
		escapedDir, escapedCmd,
	)

	// Launch as a separate wt process (no -w flag, so each is its own window)
	args := []string{
		"/c", "start", "wt",
		"--title", cfg.Title,
		"-d", cfg.WorkingDir,
		"powershell", "-NoExit", "-Command", picker,
	}

	cmd := exec.Command("cmd", args...)
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to launch terminal: %w", err)
	}

	// Wait for window to appear
	time.Sleep(500 * time.Millisecond)

	// Find and position the window
	hwnd, err := findWindowByTitle(cfg.Title)
	if err != nil {
		return fmt.Errorf("failed to find window: %w", err)
	}

	err = setWindowPosition(hwnd, cfg.X, cfg.Y, cfg.Width, cfg.Height)
	if err != nil {
		return fmt.Errorf("failed to position window: %w", err)
	}

	return nil
}

func findWindowByTitle(title string) (uintptr, error) {
	var foundHwnd uintptr

	for attempts := 0; attempts < 10; attempts++ {
		callback := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
			var windowTitle [256]uint16
			procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&windowTitle[0])), 256)

			text := syscall.UTF16ToString(windowTitle[:])
			if text == title || containsSubstring(text, title) {
				foundHwnd = hwnd
				return 0
			}
			return 1
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
