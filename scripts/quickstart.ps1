# Quickstart - Proof of Concept
# A PowerShell script to launch multiple terminals across monitors
# Each terminal opens a project picker and then runs Claude Code

param(
    [string]$ProjectsDir = "$env:USERPROFILE\.1dev",
    [string]$PostCommand = "claude --dangerously-skip-permissions",
    [int]$TotalWindows = 3,
    [switch]$Verbose
)

# ============================================================================
# Configuration - Edit these to match your setup!
# ============================================================================

$Config = @{
    # Your projects directory
    ProjectsDir = $ProjectsDir

    # Command to run after selecting a project
    PostCommand = $PostCommand

    # Monitor configuration
    # Format: @{ MonitorIndex = @{ Windows = N; Layout = "grid"|"vertical"|"horizontal" } }
    Monitors = @{
        0 = @{ Windows = 2; Layout = "vertical" }   # First monitor: 2 windows side by side
        1 = @{ Windows = 1; Layout = "full" }       # Second monitor: 1 fullscreen
        # Add more monitors as needed:
        # 2 = @{ Windows = 4; Layout = "grid" }     # Third monitor: 4 windows in grid
    }
}

# ============================================================================
# Windows API Definitions
# ============================================================================

Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Collections.Generic;

public class MonitorInfo {
    public int Left;
    public int Top;
    public int Right;
    public int Bottom;
    public bool IsPrimary;

    public int Width { get { return Right - Left; } }
    public int Height { get { return Bottom - Top; } }
}

public class WinAPI {
    [DllImport("user32.dll")]
    public static extern bool EnumDisplayMonitors(IntPtr hdc, IntPtr lprcClip, MonitorEnumDelegate lpfnEnum, IntPtr dwData);

    [DllImport("user32.dll", CharSet = CharSet.Auto)]
    public static extern bool GetMonitorInfo(IntPtr hMonitor, ref MONITORINFOEX lpmi);

    [DllImport("user32.dll", SetLastError = true)]
    public static extern IntPtr FindWindow(string lpClassName, string lpWindowName);

    [DllImport("user32.dll", SetLastError = true)]
    public static extern bool SetWindowPos(IntPtr hWnd, IntPtr hWndInsertAfter, int X, int Y, int cx, int cy, uint uFlags);

    [DllImport("user32.dll")]
    public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);

    [DllImport("user32.dll", CharSet = CharSet.Auto, SetLastError = true)]
    public static extern int GetWindowText(IntPtr hWnd, System.Text.StringBuilder lpString, int nMaxCount);

    public delegate bool MonitorEnumDelegate(IntPtr hMonitor, IntPtr hdcMonitor, ref RECT lprcMonitor, IntPtr dwData);
    public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Auto)]
    public struct MONITORINFOEX {
        public int cbSize;
        public RECT rcMonitor;
        public RECT rcWork;
        public uint dwFlags;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 32)]
        public string szDevice;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct RECT {
        public int Left;
        public int Top;
        public int Right;
        public int Bottom;
    }

    public const uint SWP_NOZORDER = 0x0004;
    public const uint SWP_SHOWWINDOW = 0x0040;
    public const int MONITORINFOF_PRIMARY = 0x00000001;

    public static List<MonitorInfo> GetMonitors() {
        var monitors = new List<MonitorInfo>();

        EnumDisplayMonitors(IntPtr.Zero, IntPtr.Zero,
            delegate (IntPtr hMonitor, IntPtr hdcMonitor, ref RECT lprcMonitor, IntPtr dwData) {
                MONITORINFOEX mi = new MONITORINFOEX();
                mi.cbSize = Marshal.SizeOf(mi);

                if (GetMonitorInfo(hMonitor, ref mi)) {
                    monitors.Add(new MonitorInfo {
                        Left = mi.rcMonitor.Left,
                        Top = mi.rcMonitor.Top,
                        Right = mi.rcMonitor.Right,
                        Bottom = mi.rcMonitor.Bottom,
                        IsPrimary = (mi.dwFlags & MONITORINFOF_PRIMARY) != 0
                    });
                }
                return true;
            }, IntPtr.Zero);

        // Sort by X position (left to right)
        monitors.Sort((a, b) => a.Left.CompareTo(b.Left));

        return monitors;
    }

    public static IntPtr FindWindowByTitle(string titlePart) {
        IntPtr found = IntPtr.Zero;

        EnumWindows(delegate (IntPtr hWnd, IntPtr lParam) {
            var sb = new System.Text.StringBuilder(256);
            GetWindowText(hWnd, sb, 256);
            if (sb.ToString().Contains(titlePart)) {
                found = hWnd;
                return false; // Stop enumeration
            }
            return true;
        }, IntPtr.Zero);

        return found;
    }
}
"@

# ============================================================================
# Functions
# ============================================================================

function Get-Monitors {
    $monitors = [WinAPI]::GetMonitors()
    return $monitors
}

function Get-WindowPositions {
    param(
        [MonitorInfo]$Monitor,
        [int]$WindowCount,
        [string]$Layout
    )

    $positions = @()

    switch ($Layout) {
        "grid" {
            # Calculate grid dimensions
            $cols = 1
            $rows = 1
            while ($cols * $rows -lt $WindowCount) {
                if ($cols -le $rows) { $cols++ } else { $rows++ }
            }

            $cellWidth = [math]::Floor($Monitor.Width / $cols)
            $cellHeight = [math]::Floor($Monitor.Height / $rows)

            for ($i = 0; $i -lt $WindowCount; $i++) {
                $row = [math]::Floor($i / $cols)
                $col = $i % $cols

                $positions += @{
                    X = $Monitor.Left + ($col * $cellWidth)
                    Y = $Monitor.Top + ($row * $cellHeight)
                    Width = $cellWidth
                    Height = $cellHeight
                }
            }
        }
        "vertical" {
            $cellWidth = [math]::Floor($Monitor.Width / $WindowCount)

            for ($i = 0; $i -lt $WindowCount; $i++) {
                $positions += @{
                    X = $Monitor.Left + ($i * $cellWidth)
                    Y = $Monitor.Top
                    Width = $cellWidth
                    Height = $Monitor.Height
                }
            }
        }
        "horizontal" {
            $cellHeight = [math]::Floor($Monitor.Height / $WindowCount)

            for ($i = 0; $i -lt $WindowCount; $i++) {
                $positions += @{
                    X = $Monitor.Left
                    Y = $Monitor.Top + ($i * $cellHeight)
                    Width = $Monitor.Width
                    Height = $cellHeight
                }
            }
        }
        "full" {
            $positions += @{
                X = $Monitor.Left
                Y = $Monitor.Top
                Width = $Monitor.Width
                Height = $Monitor.Height
            }
        }
    }

    return $positions
}

function Start-QuickstartTerminal {
    param(
        [string]$Title,
        [string]$WorkingDir,
        [hashtable]$Position,
        [string]$PostCommand
    )

    # Create the picker command that will run in the terminal
    $pickerScript = @"

# Change to projects directory
Set-Location '$WorkingDir'

# Get list of project directories
`$projects = Get-ChildItem -Directory | Select-Object -ExpandProperty Name

# Check if fzf is available
`$fzfPath = Get-Command fzf -ErrorAction SilentlyContinue

if (`$fzfPath) {
    # Use fzf for selection
    `$selected = `$projects | fzf --height=80% --reverse --border --prompt='Select project: '
} else {
    # Fallback to simple numbered menu
    Write-Host ''
    Write-Host '  Quickstart - Select a Project' -ForegroundColor Cyan
    Write-Host '  =============================' -ForegroundColor Cyan
    Write-Host ''

    for (`$i = 0; `$i -lt `$projects.Count; `$i++) {
        Write-Host "    [`$(`$i + 1)] `$(`$projects[`$i])"
    }

    Write-Host ''
    `$selection = Read-Host '  Enter number'

    `$index = [int]`$selection - 1
    if (`$index -ge 0 -and `$index -lt `$projects.Count) {
        `$selected = `$projects[`$index]
    }
}

if (`$selected) {
    `$projectPath = Join-Path '$WorkingDir' `$selected
    Set-Location `$projectPath

    Write-Host ''
    Write-Host "  Opening: `$selected" -ForegroundColor Green
    Write-Host ''

    # Run the post-select command
    $PostCommand
} else {
    Write-Host '  No project selected.' -ForegroundColor Red
    Write-Host '  Press any key to exit...'
    `$null = `$Host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown')
}
"@

    # Encode the script for passing to PowerShell
    $bytes = [System.Text.Encoding]::Unicode.GetBytes($pickerScript)
    $encodedCommand = [Convert]::ToBase64String($bytes)

    # Launch Windows Terminal
    $wtArgs = @(
        "--title", $Title,
        "-d", $WorkingDir,
        "powershell", "-NoExit", "-EncodedCommand", $encodedCommand
    )

    Start-Process "wt" -ArgumentList $wtArgs

    # Wait for window to appear
    Start-Sleep -Milliseconds 800

    # Find and position the window
    $maxAttempts = 10
    $hwnd = [IntPtr]::Zero

    for ($attempt = 0; $attempt -lt $maxAttempts; $attempt++) {
        $hwnd = [WinAPI]::FindWindowByTitle($Title)
        if ($hwnd -ne [IntPtr]::Zero) {
            break
        }
        Start-Sleep -Milliseconds 300
    }

    if ($hwnd -ne [IntPtr]::Zero) {
        $result = [WinAPI]::SetWindowPos(
            $hwnd,
            [IntPtr]::Zero,
            $Position.X,
            $Position.Y,
            $Position.Width,
            $Position.Height,
            [WinAPI]::SWP_NOZORDER -bor [WinAPI]::SWP_SHOWWINDOW
        )

        if ($Verbose) {
            Write-Host "  Positioned '$Title' at ($($Position.X), $($Position.Y)) - $($Position.Width)x$($Position.Height)"
        }
    } else {
        Write-Host "  Warning: Could not find window '$Title' to position" -ForegroundColor Yellow
    }
}

# ============================================================================
# Main
# ============================================================================

Write-Host ""
Write-Host "  Quickstart - Development Environment Launcher" -ForegroundColor Cyan
Write-Host "  =============================================" -ForegroundColor Cyan
Write-Host ""

# Detect monitors
$monitors = Get-Monitors

Write-Host "  Detected $($monitors.Count) monitor(s):" -ForegroundColor White
for ($i = 0; $i -lt $monitors.Count; $i++) {
    $m = $monitors[$i]
    $primary = if ($m.IsPrimary) { " (Primary)" } else { "" }
    Write-Host "    Monitor $($i + 1): $($m.Width)x$($m.Height) at ($($m.Left), $($m.Top))$primary"
}
Write-Host ""

# Validate projects directory
if (-not (Test-Path $Config.ProjectsDir)) {
    Write-Host "  Error: Projects directory not found: $($Config.ProjectsDir)" -ForegroundColor Red
    Write-Host "  Please edit the script and set the correct ProjectsDir" -ForegroundColor Yellow
    exit 1
}

$projectCount = (Get-ChildItem -Path $Config.ProjectsDir -Directory).Count
Write-Host "  Projects directory: $($Config.ProjectsDir) ($projectCount projects)" -ForegroundColor White
Write-Host ""

# Launch terminals
$windowIndex = 0
foreach ($monitorIndex in $Config.Monitors.Keys | Sort-Object) {
    $monitorConfig = $Config.Monitors[$monitorIndex]

    if ($monitorIndex -ge $monitors.Count) {
        Write-Host "  Skipping monitor $monitorIndex (not connected)" -ForegroundColor Yellow
        continue
    }

    $monitor = $monitors[$monitorIndex]
    $positions = Get-WindowPositions -Monitor $monitor -WindowCount $monitorConfig.Windows -Layout $monitorConfig.Layout

    Write-Host "  Monitor $($monitorIndex + 1): Launching $($monitorConfig.Windows) window(s) in $($monitorConfig.Layout) layout" -ForegroundColor Green

    foreach ($pos in $positions) {
        $windowIndex++
        $title = "Quickstart-$windowIndex"

        Start-QuickstartTerminal `
            -Title $title `
            -WorkingDir $Config.ProjectsDir `
            -Position $pos `
            -PostCommand $Config.PostCommand

        # Small delay between launches to avoid race conditions
        Start-Sleep -Milliseconds 200
    }
}

Write-Host ""
Write-Host "  Launched $windowIndex terminal window(s)" -ForegroundColor Cyan
Write-Host ""
