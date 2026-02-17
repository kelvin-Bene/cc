package window

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/bcmister/cc/internal/config"
	"github.com/bcmister/cc/internal/monitor"
)

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	procFindWindowW    = user32.NewProc("FindWindowW")
	procSetWindowPos   = user32.NewProc("SetWindowPos")
	procEnumWindows    = user32.NewProc("EnumWindows")
	procGetWindowTextW = user32.NewProc("GetWindowTextW")

	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
)

const (
	SWP_NOZORDER   = 0x0004
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
	Command    string           // e.g. "claude --dangerously-skip-permissions"
	Label      string           // e.g. "cc" or "cx"
	Profiles   []config.Profile // account profiles for picker
}

// LaunchResult holds the outcome of a terminal launch
type LaunchResult struct {
	Title string
	Err   error
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

// encodePS converts a PowerShell script to a base64 UTF-16LE encoded string
func encodePS(script string) string {
	u16 := utf16.Encode([]rune(script))
	b := make([]byte, len(u16)*2)
	for i, r := range u16 {
		b[i*2] = byte(r)
		b[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// LaunchAll launches all terminals in parallel and positions them.
// Each config carries its own Command and Label.
func LaunchAll(configs []LaunchConfig) []LaunchResult {
	results := make([]LaunchResult, len(configs))

	scripts := make([]string, len(configs))
	for i, cfg := range configs {
		scripts[i] = buildPickerScript(cfg.WorkingDir, cfg.Command, cfg.Label, cfg.Profiles)
		results[i].Title = cfg.Title
	}

	// Phase 1: Launch all wt processes as fast as possible
	for i, cfg := range configs {
		encoded := encodePS(scripts[i])
		args := []string{
			"--title", cfg.Title,
			"-d", cfg.WorkingDir,
			"powershell", "-NoExit", "-EncodedCommand", encoded,
		}
		cmd := exec.Command("wt", args...)
		if err := cmd.Start(); err != nil {
			results[i].Err = fmt.Errorf("failed to launch: %w", err)
		}
	}

	// Phase 2: Wait once for windows to start appearing, then find and position all in parallel
	time.Sleep(300 * time.Millisecond)

	var wg sync.WaitGroup
	for i, cfg := range configs {
		if results[i].Err != nil {
			continue
		}
		wg.Add(1)
		go func(idx int, c LaunchConfig) {
			defer wg.Done()
			hwnd, err := findWindowByTitle(c.Title)
			if err != nil {
				results[idx].Err = fmt.Errorf("failed to find window: %w", err)
				return
			}
			if err := setWindowPosition(hwnd, c.X, c.Y, c.Width, c.Height); err != nil {
				results[idx].Err = fmt.Errorf("failed to position: %w", err)
			}
		}(i, cfg)
	}
	wg.Wait()

	return results
}

// buildProfileArrays generates PowerShell array literals for profile names, dirs, and keys
func buildProfileArrays(profiles []config.Profile) (names, dirs, keys string) {
	if len(profiles) == 0 {
		return "@()", "@()", "@()"
	}
	nameList := make([]string, len(profiles))
	dirList := make([]string, len(profiles))
	keyList := make([]string, len(profiles))
	for i, p := range profiles {
		nameList[i] = "'" + strings.ReplaceAll(p.Name, "'", "''") + "'"
		expanded := config.ExpandPath(p.ConfigDir)
		dirList[i] = "'" + strings.ReplaceAll(expanded, "'", "''") + "'"
		keyList[i] = "'" + strings.ReplaceAll(p.APIKey, "'", "''") + "'"
	}
	return "@(" + strings.Join(nameList, ",") + ")",
		"@(" + strings.Join(dirList, ",") + ")",
		"@(" + strings.Join(keyList, ",") + ")"
}

func buildPickerScript(workingDir, command, label string, profiles []config.Profile) string {
	profileNames, profileDirs, profileKeys := buildProfileArrays(profiles)
	return `
$R   = [char]27 + '[0m'
$DIM = [char]27 + '[90m'
$CYN = [char]27 + '[96m'
$WHT = [char]27 + '[97m'
$GRN = [char]27 + '[92m'
$YEL = [char]27 + '[93m'
$RED = [char]27 + '[91m'
$INV = [char]27 + '[7m'
$HID = [char]27 + '[?25l'
$SHW = [char]27 + '[?25h'

$profileNames = ` + profileNames + `
$profileDirs  = ` + profileDirs + `
$profileKeys  = ` + profileKeys + `

$d = '` + workingDir + `'
$all = @(Get-ChildItem $d -Directory | Select-Object -ExpandProperty Name)

if ($all.Count -eq 0) {
    Write-Host ""
    Write-Host "  ${RED}No projects in $d${R}"
    Write-Host ""
    Read-Host "  Press Enter"
    exit
}

$filter = ""
$sel = 0
$viewOffset = 0
$maxShow = 12

function Draw {
    param($items, $sel, $filter, $startY, $viewOffset)

    $termH = $Host.UI.RawUI.WindowSize.Height
    $script:maxShow = [Math]::Max(1, $termH - $startY - 4)

    $maxOff = [Math]::Max(0, $items.Count - $script:maxShow)
    if ($viewOffset -gt $maxOff) { $viewOffset = $maxOff }
    $script:viewOffset = $viewOffset

    $row = $startY + 1
    Write-Host "$([char]27)[${row};1H" -NoNewline

    if ($filter -eq "") {
        Write-Host "  ${CYN}>${R} ${DIM}type to filter...${R}                    " -NoNewline
    } else {
        Write-Host "  ${CYN}>${R} ${WHT}$filter${R}                              " -NoNewline
    }
    Write-Host ""
    Write-Host "  ${DIM}─────────────────────────────────${R}      "

    for ($i = 0; $i -lt $script:maxShow; $i++) {
        $itemIdx = $viewOffset + $i
        if ($itemIdx -lt $items.Count) {
            $name = $items[$itemIdx]
            if ($itemIdx -eq $sel) {
                Write-Host "  ${INV}${CYN} > ${WHT}$name ${R}                              "
            } else {
                Write-Host "    ${DIM}$name${R}                                   "
            }
        } else {
            Write-Host "                                          "
        }
    }

    Write-Host ""
    if ($items.Count -gt $script:maxShow) {
        Write-Host "  ${DIM}↑↓${R} navigate  ${DIM}($($sel+1)/$($items.Count))${R}  ${DIM}esc${R} quit     " -NoNewline
    } else {
        Write-Host "  ${DIM}↑↓${R} navigate  ${DIM}enter${R} select  ${DIM}esc${R} quit     " -NoNewline
    }
}

function FilterList {
    param($items, $query)
    if ($query -eq "") { return $items }
    $q = $query.ToLower()
    return @($items | Where-Object { $_.ToLower().Contains($q) })
}

# Setup - ANSI clear + cursor home
Write-Host "$([char]27)[2J$([char]27)[H${HID}" -NoNewline

# 50ms delay to let terminal settle after repositioning
Start-Sleep -Milliseconds 50

Write-Host ""
Write-Host "  ${CYN}` + label + `${R} ${DIM}· select project${R}"
Write-Host ""

$startY = 3
$termH = $Host.UI.RawUI.WindowSize.Height
$maxShow = [Math]::Max(1, $termH - $startY - 4)
$filtered = $all
Draw $filtered $sel $filter $startY $viewOffset

# Main loop with try/finally for cursor restore
$readErr = 0
try {
while ($true) {
    try {
        $key = $Host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown')
        $readErr = 0
    } catch {
        $readErr++
        if ($readErr -ge 20) {
            Write-Host "${SHW}" -NoNewline
            Write-Host ""
            Write-Host "  ${RED}Input error${R}"
            exit 1
        }
        Start-Sleep -Milliseconds 50
        continue
    }
    $vk = $key.VirtualKeyCode
    $ch = $key.Character

    # Escape - quit
    if ($vk -eq 27) {
        Write-Host "${SHW}" -NoNewline
        Clear-Host
        exit
    }

    # Enter - select
    if ($vk -eq 13) {
        if ($filtered.Count -gt 0) {
            $chosen = $filtered[$sel]
            Write-Host "${SHW}" -NoNewline
            Clear-Host
            Write-Host ""
            Write-Host "  ${GRN}>${R} ${WHT}$chosen${R}"
            Write-Host ""

            # Account picker phase
            if ($profileNames.Count -gt 1) {
                Write-Host "  ${CYN}` + label + `${R} ${DIM}· select account${R}"
                Write-Host "  ${DIM}─────────────────────────────────${R}"
                for ($pi = 0; $pi -lt $profileNames.Count; $pi++) {
                    $num = $pi + 1
                    Write-Host "  ${WHT}${num}${R}  ${DIM}$($profileNames[$pi])${R}"
                }
                Write-Host ""
                Write-Host "  ${CYN}>${R} " -NoNewline

                $picked = $false
                while (-not $picked) {
                    try {
                        $aKey = $Host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown')
                    } catch { continue }
                    $aVk = $aKey.VirtualKeyCode
                    if ($aVk -eq 27) {
                        Write-Host "${SHW}" -NoNewline
                        Clear-Host
                        exit
                    }
                    $aNum = $aVk - 48
                    if ($aNum -ge 1 -and $aNum -le $profileNames.Count) {
                        $idx = $aNum - 1
                        $env:CLAUDE_CONFIG_DIR = $profileDirs[$idx]
                        if ($profileKeys[$idx] -ne '') {
                            $env:ANTHROPIC_API_KEY = $profileKeys[$idx]
                        }
                        Write-Host "$($profileNames[$idx])"
                        Write-Host ""
                        $picked = $true
                    }
                }
            } elseif ($profileNames.Count -eq 1) {
                $env:CLAUDE_CONFIG_DIR = $profileDirs[0]
                if ($profileKeys[0] -ne '') {
                    $env:ANTHROPIC_API_KEY = $profileKeys[0]
                }
            }

            Set-Location (Join-Path $d $chosen)
            ` + command + `
            break
        }
    }

    # Up arrow
    if ($vk -eq 38) {
        if ($sel -gt 0) {
            $sel--
            if ($sel -lt $viewOffset) { $viewOffset = $sel }
        }
        Draw $filtered $sel $filter $startY $viewOffset
        continue
    }

    # Down arrow
    if ($vk -eq 40) {
        if ($sel -lt ($filtered.Count - 1)) {
            $sel++
            if ($sel -ge ($viewOffset + $maxShow)) { $viewOffset = $sel - $maxShow + 1 }
        }
        Draw $filtered $sel $filter $startY $viewOffset
        continue
    }

    # Backspace
    if ($vk -eq 8) {
        if ($filter.Length -gt 0) {
            $filter = $filter.Substring(0, $filter.Length - 1)
            $filtered = @(FilterList $all $filter)
            $sel = 0
            $viewOffset = 0
            Draw $filtered $sel $filter $startY $viewOffset
        }
        continue
    }

    # Printable character - add to filter
    $code = [int]$ch
    if ($code -gt 32 -and $code -le 126) {
        $filter += $ch
        $filtered = @(FilterList $all $filter)
        $sel = 0
        $viewOffset = 0
        Draw $filtered $sel $filter $startY $viewOffset
    }
}
} finally {
    Write-Host "${SHW}" -NoNewline
}
`
}

// LaunchTab opens a new tab in the current Windows Terminal window
func LaunchTab(workingDir, command, label string, profiles []config.Profile) error {
	script := buildPickerScript(workingDir, command, label, profiles)
	encoded := encodePS(script)
	args := []string{
		"-w", "0",
		"new-tab",
		"-d", workingDir,
		"powershell", "-NoExit", "-EncodedCommand", encoded,
	}
	cmd := exec.Command("wt", args...)
	return cmd.Start()
}

// LaunchTerminal launches a single terminal (kept for backward compat)
func LaunchTerminal(cfg LaunchConfig) error {
	results := LaunchAll([]LaunchConfig{cfg})
	return results[0].Err
}

func findWindowByTitle(title string) (uintptr, error) {
	var foundHwnd uintptr

	// Poll at 50ms intervals instead of 200ms — find window as soon as it appears
	for attempts := 0; attempts < 40; attempts++ {
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

		time.Sleep(50 * time.Millisecond)
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

// GetCurrentConsoleWindow returns the HWND of the current console window
func GetCurrentConsoleWindow() uintptr {
	hwnd, _, _ := procGetConsoleWindow.Call()
	return hwnd
}

// RunPickerInCurrent runs the picker script in the current terminal (blocking).
// Bug fix: removed -NoExit so the process exits cleanly after picker selection.
func RunPickerInCurrent(workingDir, command, label string, profiles []config.Profile) error {
	script := buildPickerScript(workingDir, command, label, profiles)
	encoded := encodePS(script)

	cmd := exec.Command("powershell", "-EncodedCommand", encoded)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// LaunchAllWithCurrentResult holds the results and a picker function
type LaunchAllWithCurrentResult struct {
	Results   []LaunchResult
	RunPicker func() error
}

// LaunchAllWithCurrent launches terminals where index 0 uses the current terminal
// and indexes 1+ spawn new windows. Each config carries its own Command and Label.
func LaunchAllWithCurrent(configs []LaunchConfig) LaunchAllWithCurrentResult {
	if len(configs) == 0 {
		return LaunchAllWithCurrentResult{
			Results:   nil,
			RunPicker: func() error { return nil },
		}
	}

	results := make([]LaunchResult, len(configs))
	for i, cfg := range configs {
		results[i].Title = cfg.Title
	}

	// Get current console window handle for positioning
	currentHwnd := GetCurrentConsoleWindow()

	// Launch additional windows (configs[1:]) via wt — each with its own command/label
	if len(configs) > 1 {
		for i := 1; i < len(configs); i++ {
			cfg := configs[i]
			script := buildPickerScript(cfg.WorkingDir, cfg.Command, cfg.Label, cfg.Profiles)
			encoded := encodePS(script)
			args := []string{
				"--title", cfg.Title,
				"-d", cfg.WorkingDir,
				"powershell", "-NoExit", "-EncodedCommand", encoded,
			}
			cmd := exec.Command("wt", args...)
			if err := cmd.Start(); err != nil {
				results[i].Err = fmt.Errorf("failed to launch: %w", err)
			}
		}

		// Wait for windows to appear
		time.Sleep(300 * time.Millisecond)
	}

	// Position all windows in parallel
	var wg sync.WaitGroup

	// Position current terminal (index 0)
	if currentHwnd != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := configs[0]
			if err := setWindowPosition(currentHwnd, cfg.X, cfg.Y, cfg.Width, cfg.Height); err != nil {
				results[0].Err = fmt.Errorf("failed to position current window: %w", err)
			}
		}()
	}

	// Position spawned windows (configs[1:])
	for i := 1; i < len(configs); i++ {
		if results[i].Err != nil {
			continue
		}
		wg.Add(1)
		go func(idx int, cfg LaunchConfig) {
			defer wg.Done()
			hwnd, err := findWindowByTitle(cfg.Title)
			if err != nil {
				results[idx].Err = fmt.Errorf("failed to find window: %w", err)
				return
			}
			if err := setWindowPosition(hwnd, cfg.X, cfg.Y, cfg.Width, cfg.Height); err != nil {
				results[idx].Err = fmt.Errorf("failed to position: %w", err)
			}
		}(i, configs[i])
	}
	wg.Wait()

	// Return results and a picker function — uses first config's command/label
	picker := func() error {
		return RunPickerInCurrent(configs[0].WorkingDir, configs[0].Command, configs[0].Label, configs[0].Profiles)
	}

	return LaunchAllWithCurrentResult{
		Results:   results,
		RunPicker: picker,
	}
}
