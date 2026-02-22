package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/revlat/pings/pkg/pinger"
)

func clearConsole() {
	cmd := exec.Command("clear")
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func printHelp() {
	fmt.Println(`NAME
    pings - monitor multiple hosts with continuous ping checks

USAGE
    pings [OPTIONS] IP [IP ...] [INTERVAL]
    pings [OPTIONS] FILE [INTERVAL]

DESCRIPTION
    Continuously pings multiple IP addresses and displays their online/offline
    status with success/failure counters, latency measurements, uptime percentage,
    and status change tracking.

OPTIONS
    -h, --help           Show this help message
    -s, --sort MODE      Sort output by status (MODE: online, offline)
                         - online:  Show online hosts first
                         - offline: Show offline hosts first
                         - Default: No sorting (original order)

ARGUMENTS
    IP              One or more IP addresses to monitor
    FILE            Path to a file containing IP addresses (one per line)
    INTERVAL        Scan interval in seconds (default: 5)

EXAMPLES
    Monitor multiple IPs with default 5s interval:
      pings 192.168.1.1 192.168.1.2

    Monitor with custom 10s interval:
      pings 192.168.1.1 192.168.1.2 10

    Sort with offline hosts first:
      pings -s offline 192.168.1.1 192.168.1.2

    Read IPs from file with online hosts first:
      pings --sort online ip-list.txt

    Read IPs from file with custom 15s interval:
      pings ip-list.txt 15`)
}

func main() {
	var ips []string
	interval := 5
	sortMode := ""

	if len(os.Args) < 2 {
		fmt.Println("Error: No IP addresses or file specified")
		printHelp()
		return
	}

	args := []string{}
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-h" || arg == "--help" {
			printHelp()
			return
		} else if arg == "-s" || arg == "--sort" {
			if i+1 < len(os.Args) {
				sortMode = os.Args[i+1]
				if sortMode != "online" && sortMode != "offline" {
					fmt.Printf("Error: Invalid sort mode '%s'. Use 'online' or 'offline'\n", sortMode)
					printHelp()
					return
				}
				i++
			} else {
				fmt.Println("Error: --sort requires an argument (online or offline)")
				printHelp()
				return
			}
		} else {
			args = append(args, arg)
		}
	}

	if len(args) == 0 {
		fmt.Println("Error: No IP addresses or file specified")
		printHelp()
		return
	}

	if _, err := os.Stat(args[0]); err == nil {
		var err error
		ips, err = pinger.LoadIPsFromFile(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
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

	monitor := pinger.NewMonitor()
	monitor.Start(ips, interval, func(states []pinger.HostState) {
		displayStates := make([]pinger.HostState, len(states))
		copy(displayStates, states)

		if sortMode != "" {
			sort.SliceStable(displayStates, func(i, j int) bool {
				if displayStates[i].Online != displayStates[j].Online {
					if sortMode == "offline" {
						return !displayStates[i].Online
					}
					return displayStates[i].Online
				}
				return false
			})
		}

		var output strings.Builder
		output.WriteString("\n--- IP Status Check ---\n")

		for _, state := range displayStates {
			status := "Offline"
			if state.Online {
				status = "Online"
			}

			statusColor := "\033[91m"
			if state.Online {
				statusColor = "\033[92m"
			}

			totalPings := state.OkCount + state.FailCount
			var uptimePercent float64
			if totalPings > 0 {
				uptimePercent = float64(state.OkCount) / float64(totalPings) * 100
			}

			counterFormat := fmt.Sprintf("(%3d ok / %3d !ok)", state.OkCount, state.FailCount)

			var latencyStr string
			if state.Online && state.Latency > 0 {
				latencyStr = fmt.Sprintf("%4dms", state.Latency.Milliseconds())
			} else {
				latencyStr = "  -- "
			}

			var uptimeStr string
			if totalPings > 0 {
				uptimeStr = fmt.Sprintf("%5.1f%%", uptimePercent)
			} else {
				uptimeStr = "  -- "
			}

			if state.FailCount > 0 {
				counterFormat = fmt.Sprintf("\033[93m%s\033[0m", counterFormat)
			}

			output.WriteString(fmt.Sprintf("%-18s  %s%-8s\033[0m  %s  %s  %s up",
				state.IP, statusColor, status, counterFormat, latencyStr, uptimeStr))

			if !state.LastStatusChange.IsZero() {
				timeSince := time.Since(state.LastStatusChange)
				changeLabel := "up"
				if state.Online {
					changeLabel = "down"
				}
				output.WriteString(fmt.Sprintf("  (%s: %s)", changeLabel, pinger.FormatTimeSince(timeSince)))
			}
			output.WriteString("\n")
		}

		clearConsole()
		fmt.Print(output.String())
	})
}
