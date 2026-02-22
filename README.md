# pings

🇬🇧 [English](README.md) | 🇩🇪 [Deutsch](README_DE.md)

A lightweight tool for monitoring multiple hosts with continuous ping checks and real-time status updates — available as a **CLI** and a **graphical GUI**.

## Features

- 🎯 **Multi-host monitoring** - Ping multiple IP addresses simultaneously
- ⚡ **Concurrent pings** - All hosts are pinged in parallel for fast results
- 📊 **Success/failure tracking** - Count successful and failed pings over time
- ⏱️ **Latency measurement** - Display round-trip time of the last successful ping
- 📈 **Uptime percentage** - Calculate availability over time
- 🔄 **Status change tracking** - Show when hosts went online/offline
- 🎨 **Color-coded output** - Green for online, red for offline, yellow warnings
- 📁 **File input support** - Read IP addresses from a file
- ⚙️ **Configurable interval** - Adjust scan frequency (default: 5 seconds)
- 🖥️ **Cross-platform** - Works on Windows, Linux, and macOS
- 🪟 **GUI included** - Optional graphical interface with sortable table (`pings-gui`)

## Installation

### Download Precompiled Binaries

Download the latest release for your platform from the [Releases page](https://github.com/revlat/pings/releases).

**CLI (`pings`)** — pure Go, no dependencies:

| Platform | Archive | Binary |
|----------|---------|--------|
| Windows x64 | `pings-windows-amd64.zip` | `pings.exe` |
| Windows ARM64 | `pings-windows-arm64.zip` | `pings.exe` |
| Linux x64 | `pings-linux-amd64.tar.gz` | `pings` |
| Linux ARM64 | `pings-linux-arm64.tar.gz` | `pings` |
| macOS Intel | `pings-darwin-amd64.tar.gz` | `pings` |
| macOS Apple Silicon | `pings-darwin-arm64.tar.gz` | `pings` |

**GUI (`pings-gui`)** — requires a display:

| Platform | Archive | Binary |
|----------|---------|--------|
| Windows x64 | `pings-gui-windows-amd64.zip` | `pings-gui.exe` |
| Linux x64 | `pings-gui-linux-amd64.tar.gz` | `pings-gui` |
| Linux ARM64 | `pings-gui-linux-arm64.tar.gz` | `pings-gui` |
| macOS Apple Silicon | `pings-gui-darwin-arm64.tar.gz` | `pings-gui` |

Extract the archive and run the binary directly.

### Build from Source

Requires [Go 1.24+](https://go.dev/dl/):

```sh
git clone https://github.com/revlat/pings.git
cd pings
make build        # CLI binary for current OS
make build-gui    # GUI binary for current OS (see dependencies below)
```

## CLI Usage

```
pings [OPTIONS] IP [IP ...] [INTERVAL]
pings [OPTIONS] FILE [INTERVAL]
```

Run `pings --help` for full usage information.

### Examples

**Monitor multiple IPs with default 5-second interval:**
```sh
pings 1.1.1.1 8.8.8.8 9.9.9.9
```

**Monitor with custom 10-second interval:**
```sh
pings 192.168.1.1 192.168.1.254 10
```

**Sort with offline hosts first:**
```sh
pings -s offline 192.168.1.1 192.168.1.2 192.168.1.3
```

**Read IPs from a file:**
```sh
pings ip-list.txt
```

## CLI Output Example

![CLI Output](example.png)

The output shows:
- **Green "Online"** / **Red "Offline"** - Current reachability
- **Counters** - `(5 ok / 0 !ok)` tracks successful and failed pings over time
- **Latency** - `12ms` round-trip time of the last successful ping (or `--` if offline)
- **Uptime** - `98.5% up` availability percentage since monitoring started
- **Status change** - `(down: 5s ago)` / `(up: 2m 30s ago)` — time since last status flip
- **Yellow highlighting** - At least one failed ping for that host
- **Sorting** - `-s offline` / `-s online` to reorder hosts

## pings-gui

`pings-gui` is a graphical frontend with the same monitoring engine as the CLI. It opens a window with a live-updating table.

### GUI Usage

`pings-gui` accepts the same arguments as `pings`.

```
pings-gui [OPTIONS] IP [IP ...] [INTERVAL]
pings-gui [OPTIONS] FILE [INTERVAL]
```

When launched **without arguments**, a setup form is shown where you can enter IP addresses and the scan interval interactively:

![GUI Setup Form](example-gui-start.png)

After clicking **Start** (or passing IPs as arguments), the monitoring table opens:

![GUI Monitoring Table](example-gui-run.png)

The table columns can be **sorted by clicking any header** — click again to reverse the order:

| Column | Description |
|--------|-------------|
| IP | Host address |
| Status | Online (green) / Offline (red) |
| OK | Successful ping count |
| !OK | Failed ping count (yellow if > 0) |
| Latency | Last round-trip time |
| Uptime | Availability percentage |
| Last Change | Time since last status change |

### Building pings-gui

`pings-gui` uses [Fyne](https://fyne.io/) and requires **CGO** (a C compiler). Install the required system libraries first:

**Linux (Debian/Ubuntu):**
```sh
sudo apt-get install libgl1-mesa-dev libx11-dev libxrandr-dev libxcursor-dev \
  libxinerama-dev libxi-dev libxxf86vm-dev
```

**Linux (openSUSE):**
```sh
sudo zypper install libX11-devel Mesa-libGL-devel libXrandr-devel \
  libXcursor-devel libXinerama-devel libXi-devel libXxf86vm-devel
```

**Windows:** Install [MinGW-w64](https://www.mingw-w64.org/) (e.g. via MSYS2 or TDM-GCC) to provide `gcc`.

**macOS:** Xcode Command Line Tools are sufficient:
```sh
xcode-select --install
```

Then build:
```sh
make build-gui
```

## Notes

- **Windows users (CLI)**: Use **PowerShell** or **Windows Terminal** for color support. `cmd.exe` may not display ANSI colors correctly.
- The CLI clears the console between updates to provide a clean, real-time view.

## Building for Multiple Platforms

Using the included `Makefile`:

```sh
make build          # CLI for current OS
make build-gui      # GUI for current OS
make build-all      # CLI for all platforms (output in build/)
make build-all-gui  # GUI for current OS into build/
make clean          # Clean build artifacts
```

## Technical Details

### Concurrent Ping Implementation

The tool uses **Go's goroutines** to ping all hosts in parallel, providing fast results even when monitoring many hosts:

- **Sequential approach**: 20 hosts × 1s timeout = ~20 seconds per scan
- **Concurrent approach**: 20 hosts in parallel = ~1-2 seconds per scan

All ping operations run concurrently, with thread-safe counters protected by mutex locks.

### Why String Parsing on Windows?

On **Linux/macOS**, the `ping` command reliably returns:
- Exit code `0` = Host is reachable
- Exit code `≠ 0` = Host is unreachable

On **Windows**, `ping.exe` returns **exit code 0 even when the host is unreachable** if an ICMP error response is received (e.g., "Destination net unreachable" from a router). Example:

```
C:\> ping 10.15.15.15
Reply from <router>: Destination net unreachable.
Exit code: 0
```

Therefore, this tool uses different detection methods per platform:

- **Windows**: Output parsing to detect actual reachability
  - Checks for successful reply: `"Reply from"` (EN) or `"Antwort von"` (DE)
  - Checks for error messages: `"unreachable"`, `"timed out"`, etc.
  - Supports both English and German Windows installations

- **Linux/macOS**: Exit code evaluation (reliable)

## License

MIT License - See [LICENSE](LICENSE) file for details.
