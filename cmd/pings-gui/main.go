package main

import (
	"fmt"
	"image/color"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/revlat/pings/pkg/pinger"
)

var headers = []string{"IP", "Status", "OK", "!OK", "Latency", "Uptime", "Last Change"}

func printHelp() {
	fmt.Fprintf(os.Stderr, `NAME
    pings-gui - monitor multiple hosts with a graphical interface

USAGE
    pings-gui [OPTIONS] IP [IP ...] [INTERVAL]
    pings-gui [OPTIONS] FILE [INTERVAL]

OPTIONS
    -h, --help     Show this help message

ARGUMENTS
    IP              One or more IP addresses to monitor
    FILE            Path to a file containing IP addresses (one per line)
    INTERVAL        Scan interval in seconds (default: 5)

EXAMPLES
    pings-gui 192.168.1.1 192.168.1.2
    pings-gui 192.168.1.1 192.168.1.2 10
    pings-gui ip-list.txt 15
`)
}

// parseArgs parses command-line arguments. Returns ok=false if no args were
// given (caller should show setup UI) or on error (already printed).
func parseArgs() (ips []string, interval int, ok bool) {
	interval = 5

	args := []string{}
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-h" || arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		args = append(args, arg)
	}

	if len(args) == 0 {
		return nil, 0, false
	}

	if _, err := os.Stat(args[0]); err == nil {
		loaded, err := pinger.LoadIPsFromFile(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return nil, 0, false
		}
		ips = loaded
		if len(args) > 1 {
			if i, err := strconv.Atoi(args[1]); err == nil {
				interval = i
			}
		}
	} else {
		ips = args
		if len(ips) > 1 {
			if i, err := strconv.Atoi(ips[len(ips)-1]); err == nil {
				interval = i
				ips = ips[:len(ips)-1]
			}
		}
	}

	return ips, interval, true
}

// parseSetupInput parses the raw text from the setup form.
func parseSetupInput(ipText, intervalText string) (ips []string, interval int) {
	interval = 5
	if i, err := strconv.Atoi(strings.TrimSpace(intervalText)); err == nil && i > 0 {
		interval = i
	}

	for _, line := range strings.Split(ipText, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			ips = append(ips, line)
		}
	}

	// Einzelner Eintrag der auf eine Datei zeigt → aus Datei laden
	if len(ips) == 1 {
		if loaded, err := pinger.LoadIPsFromFile(ips[0]); err == nil {
			ips = loaded
		}
	}

	return ips, interval
}

// showSetup zeigt das Eingabe-Formular wenn keine Parameter übergeben wurden.
func showSetup(w fyne.Window) {
	ipEntry := widget.NewMultiLineEntry()
	ipEntry.SetPlaceHolder("192.168.1.1\n192.168.1.2\n...\noder Pfad zu einer Datei")
	ipEntry.SetMinRowsVisible(5)

	intervalEntry := widget.NewEntry()
	intervalEntry.SetText("5")
	intervalEntry.Validator = func(s string) error {
		if _, err := strconv.Atoi(strings.TrimSpace(s)); err != nil {
			return fmt.Errorf("nur Zahlen erlaubt")
		}
		return nil
	}

	form := widget.NewForm(
		widget.NewFormItem("IP-Adressen / Datei", ipEntry),
		widget.NewFormItem("Intervall (Sekunden)", intervalEntry),
	)

	startBtn := widget.NewButton("Start", func() {
		ips, interval := parseSetupInput(ipEntry.Text, intervalEntry.Text)
		if len(ips) == 0 {
			dialog.ShowError(fmt.Errorf("bitte mindestens eine IP-Adresse eingeben"), w)
			return
		}
		startMonitoring(w, ips, interval)
	})
	startBtn.Importance = widget.HighImportance

	content := container.NewPadded(container.NewVBox(form, startBtn))
	w.SetContent(content)
	w.Resize(fyne.NewSize(400, content.MinSize().Height))
	w.CenterOnScreen()
}

func applySortedStates(states []pinger.HostState, col int, asc bool) []pinger.HostState {
	result := make([]pinger.HostState, len(states))
	copy(result, states)
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		var less bool
		switch col {
		case 0:
			less = a.IP < b.IP
		case 1:
			less = a.Online && !b.Online
		case 2:
			less = a.OkCount < b.OkCount
		case 3:
			less = a.FailCount < b.FailCount
		case 4:
			less = a.Latency < b.Latency
		case 5:
			totalA := a.OkCount + a.FailCount
			totalB := b.OkCount + b.FailCount
			var upA, upB float64
			if totalA > 0 {
				upA = float64(a.OkCount) / float64(totalA)
			}
			if totalB > 0 {
				upB = float64(b.OkCount) / float64(totalB)
			}
			less = upA < upB
		case 6:
			less = a.LastStatusChange.Before(b.LastStatusChange)
		}
		if asc {
			return less
		}
		return !less
	})
	return result
}

// startMonitoring ersetzt den Fensterinhalt durch die Monitoring-Tabelle.
func startMonitoring(w fyne.Window, ips []string, interval int) {
	w.SetTitle(fmt.Sprintf("pings-gui  (interval: %ds)", interval))

	var mu sync.Mutex
	var currentStates []pinger.HostState
	sortCol := -1
	sortAsc := true

	colorOnline := color.NRGBA{R: 0, G: 180, B: 80, A: 255}
	colorOffline := color.NRGBA{R: 220, G: 50, B: 50, A: 255}
	colorWarn := color.NRGBA{R: 220, G: 170, B: 0, A: 255}

	var table *widget.Table
	table = widget.NewTable(
		func() (int, int) {
			mu.Lock()
			rows := len(currentStates) + 1
			mu.Unlock()
			return rows, len(headers)
		},
		func() fyne.CanvasObject {
			text := canvas.NewText("", theme.ForegroundColor())
			text.TextSize = 13
			btn := widget.NewButton("", nil)
			btn.Importance = widget.LowImportance
			btn.Hide()
			return container.NewStack(text, btn)
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			c := obj.(*fyne.Container)
			text := c.Objects[0].(*canvas.Text)
			btn := c.Objects[1].(*widget.Button)

			if id.Row == 0 {
				text.Hide()

				mu.Lock()
				sc, sa := sortCol, sortAsc
				mu.Unlock()

				indicator := ""
				if id.Col == sc {
					if sa {
						indicator = " ▲"
					} else {
						indicator = " ▼"
					}
				}
				btn.SetText(headers[id.Col] + indicator)

				col := id.Col
				btn.OnTapped = func() {
					mu.Lock()
					if sortCol == col {
						sortAsc = !sortAsc
					} else {
						sortCol = col
						sortAsc = true
					}
					currentStates = applySortedStates(currentStates, sortCol, sortAsc)
					mu.Unlock()
					table.Refresh()
				}
				btn.Show()
				return
			}

			btn.Hide()
			text.Show()
			text.TextStyle = fyne.TextStyle{}
			text.Color = theme.ForegroundColor()

			mu.Lock()
			if id.Row-1 >= len(currentStates) {
				mu.Unlock()
				text.Text = ""
				return
			}
			state := currentStates[id.Row-1]
			mu.Unlock()

			switch id.Col {
			case 0:
				text.Text = state.IP
			case 1:
				if state.Online {
					text.Text = "Online"
					text.Color = colorOnline
				} else {
					text.Text = "Offline"
					text.Color = colorOffline
				}
			case 2:
				text.Text = fmt.Sprintf("%d", state.OkCount)
			case 3:
				text.Text = fmt.Sprintf("%d", state.FailCount)
				if state.FailCount > 0 {
					text.Color = colorWarn
				}
			case 4:
				if state.Online && state.Latency > 0 {
					text.Text = fmt.Sprintf("%dms", state.Latency.Milliseconds())
				} else {
					text.Text = "--"
				}
			case 5:
				total := state.OkCount + state.FailCount
				if total > 0 {
					uptime := float64(state.OkCount) / float64(total) * 100
					text.Text = fmt.Sprintf("%.1f%%", uptime)
				} else {
					text.Text = "--"
				}
			case 6:
				if !state.LastStatusChange.IsZero() {
					text.Text = pinger.FormatTimeSince(time.Since(state.LastStatusChange))
				} else {
					text.Text = "--"
				}
			}
		},
	)

	table.SetColumnWidth(0, 160)
	table.SetColumnWidth(1, 80)
	table.SetColumnWidth(2, 60)
	table.SetColumnWidth(3, 60)
	table.SetColumnWidth(4, 80)
	table.SetColumnWidth(5, 80)
	table.SetColumnWidth(6, 160)

	monitor := pinger.NewMonitor()
	go monitor.Start(ips, interval, func(states []pinger.HostState) {
		mu.Lock()
		if sortCol >= 0 {
			currentStates = applySortedStates(states, sortCol, sortAsc)
		} else {
			currentStates = states
		}
		mu.Unlock()
		fyne.Do(func() {
			table.Refresh()
		})
	})

	w.SetContent(container.NewBorder(nil, nil, nil, nil, table))
	w.Resize(fyne.NewSize(700, float32(40+len(ips)*30)))
}

func main() {
	a := app.New()
	w := a.NewWindow("pings-gui")

	if len(os.Args) >= 2 {
		ips, interval, ok := parseArgs()
		if !ok {
			return
		}
		startMonitoring(w, ips, interval)
	} else {
		showSetup(w)
	}

	w.ShowAndRun()
}
