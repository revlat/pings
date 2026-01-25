package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"bytes"
)

// clearConsole löscht den Konsoleninhalt
func clearConsole() {
	cmd := exec.Command("clear")
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// pingHost führt einen Ping zu einer IP aus und gibt Erfolg und Latenz zurück
func pingHost(ip string) (bool, time.Duration) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "1000", ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	start := time.Now()
	err := cmd.Run()
	latency := time.Since(start)

	if runtime.GOOS == "windows" {
		// Windows: Exit-Code ist unzuverlässig (gibt 0 zurück auch bei "Zielnetz nicht erreichbar")
		// Deshalb String-Parsing der Ausgabe notwendig
		if err != nil {
			return false, 0
		}

		output := out.String()

		// Prüfe auf erfolgreiche Antwort (Deutsch oder Englisch)
		hasReply := strings.Contains(output, "Antwort von") ||
		            strings.Contains(output, "Reply from")

		// Prüfe auf Fehlermeldungen (Deutsch oder Englisch)
		hasError := strings.Contains(output, "nicht erreichbar") ||
		            strings.Contains(output, "Zeitüberschreitung") ||
		            strings.Contains(output, "unreachable") ||
		            strings.Contains(output, "timed out")

		if hasReply && !hasError {
			return true, latency
		}
		return false, 0
	} else {
		// Linux/macOS: Exit-Code ist zuverlässig (0 = erfolgreich, !=0 = fehlgeschlagen)
		if err == nil {
			return true, latency
		}
		return false, 0
	}
}

// loadIPsFromFile lädt die IP-Adressen aus einer Datei
func loadIPsFromFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Error: File %s not found", filename)
	}
	defer file.Close()

	var ips []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			ips = append(ips, line)
		}
	}
	return ips, scanner.Err()
}

// formatTimeSince formatiert eine Zeitdauer in lesbares Format
func formatTimeSince(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds ago", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds ago", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds ago", seconds)
	}
}

// printHelp displays the help message
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
	interval := 5 // Standardintervall in Sekunden
	sortMode := "" // Sortierung: "online", "offline", oder "" (keine Sortierung)
	pingCounts := make(map[string]int)          // Erfolgreiche Pings
	failedCounts := make(map[string]int)        // Fehlgeschlagene Pings
	latencies := make(map[string]time.Duration) // Letzte Latenz
	lastSuccess := make(map[string]time.Time)   // Zeitpunkt des letzten erfolgreichen Pings
	lastStatus := make(map[string]bool)         // Letzter Ping-Status (true=online)
	previousStatus := make(map[string]*bool)    // Vorheriger Status (nil = noch keine Historie)
	lastStatusChange := make(map[string]time.Time) // Zeitpunkt der letzten Statusänderung

	// Argument-Parsing
	if len(os.Args) < 2 {
		fmt.Println("Error: No IP addresses or file specified")
		printHelp()
		return
	}

	// Parse flags und sammle nicht-flag Argumente
	args := []string{}
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]

		if arg == "-h" || arg == "--help" {
			printHelp()
			return
		} else if arg == "-s" || arg == "--sort" {
			// Nächstes Argument ist der Sort-Modus
			if i+1 < len(os.Args) {
				sortMode = os.Args[i+1]
				if sortMode != "online" && sortMode != "offline" {
					fmt.Printf("Error: Invalid sort mode '%s'. Use 'online' or 'offline'\n", sortMode)
					printHelp()
					return
				}
				i++ // Überspringe das nächste Argument (Sort-Modus)
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

	// Prüfen, ob das erste Argument eine Datei ist
	if _, err := os.Stat(args[0]); err == nil {
		// Falls Datei existiert, lade IPs aus Datei
		ips, err = loadIPsFromFile(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		// Falls noch ein weiteres Argument (Intervall) vorhanden ist
		if len(args) > 1 {
			if i, err := strconv.Atoi(args[1]); err == nil {
				interval = i
			}
		}
	} else {
		// Falls keine Datei, dann als einzelne IPs behandeln
		ips = args
		// Prüfen, ob das letzte Argument eine Zahl ist, um es als Intervall zu verwenden
		if len(ips) > 1 {
			if i, err := strconv.Atoi(ips[len(ips)-1]); err == nil {
				interval = i
				ips = ips[:len(ips)-1] // Entferne das Intervall aus der IP-Liste
			}
		}
	}

	// Mutex für thread-safe Zugriff auf Maps
	var mu sync.Mutex

	// Ping-Ergebnis Struktur
	type PingResult struct {
		ip      string
		success bool
		latency time.Duration
	}

	for {
		// Channel für Ping-Ergebnisse
		results := make(chan PingResult, len(ips))
		var wg sync.WaitGroup

		// Alle Hosts parallel pingen
		for _, ip := range ips {
			wg.Add(1)
			go func(host string) {
				defer wg.Done()
				success, latency := pingHost(host)
				results <- PingResult{host, success, latency}
			}(ip)
		}

		// Warte auf alle Goroutines
		go func() {
			wg.Wait()
			close(results)
		}()

		// Ergebnisse verarbeiten
		for result := range results {
			mu.Lock()

			// Prüfe ob Status sich geändert hat
			currentStatus := result.success
			if previousStatus[result.ip] != nil && *previousStatus[result.ip] != currentStatus {
				// Status hat sich geändert
				lastStatusChange[result.ip] = time.Now()
			}

			if result.success {
				pingCounts[result.ip]++
				latencies[result.ip] = result.latency
				lastSuccess[result.ip] = time.Now()
				lastStatus[result.ip] = true
			} else {
				failedCounts[result.ip]++
				lastStatus[result.ip] = false
			}

			// Speichere aktuellen Status als vorherigen für nächste Iteration
			statusCopy := currentStatus
			previousStatus[result.ip] = &statusCopy

			mu.Unlock()
		}

		// Ausgabe generieren
		var output strings.Builder
		output.WriteString("\n--- IP Status Check ---\n")

		// Sortiere IPs falls gewünscht
		displayIPs := make([]string, len(ips))
		copy(displayIPs, ips)
		if sortMode != "" {
			mu.Lock()
			sort.Slice(displayIPs, func(i, j int) bool {
				statusI := lastStatus[displayIPs[i]]
				statusJ := lastStatus[displayIPs[j]]

				if sortMode == "offline" {
					// Offline zuerst (false vor true)
					if statusI != statusJ {
						return !statusI // false (offline) kommt zuerst
					}
				} else if sortMode == "online" {
					// Online zuerst (true vor false)
					if statusI != statusJ {
						return statusI // true (online) kommt zuerst
					}
				}
				// Bei gleichem Status: Original-Reihenfolge beibehalten
				return false
			})
			mu.Unlock()
		}

		for _, ip := range displayIPs {
			mu.Lock()
			okCount := pingCounts[ip]
			failCount := failedCounts[ip]
			lat := latencies[ip]
			isOnline := lastStatus[ip]
			statusChangeTime := lastStatusChange[ip]
			mu.Unlock()

			status := "Offline"
			if isOnline {
				status = "Online"
			}

			// Farbige Statusausgabe
			statusColor := "\033[91m" // Rot für Offline
			if status == "Online" {
				statusColor = "\033[92m" // Grün für Online
			}

			// Berechne Uptime-Prozent
			totalPings := okCount + failCount
			var uptimePercent float64
			if totalPings > 0 {
				uptimePercent = float64(okCount) / float64(totalPings) * 100
			}

			// Counter mit Latenz und Uptime (einheitliche Breite mit festen Spalten)
			counterFormat := fmt.Sprintf("(%3d ok / %3d !ok)", okCount, failCount)

			// Latenz mit fester Breite (6 Zeichen)
			var latencyStr string
			if isOnline && lat > 0 {
				latencyStr = fmt.Sprintf("%4dms", lat.Milliseconds())
			} else {
				latencyStr = "  -- "
			}

			// Uptime mit fester Breite
			var uptimeStr string
			if totalPings > 0 {
				uptimeStr = fmt.Sprintf("%5.1f%%", uptimePercent)
			} else {
				uptimeStr = "  -- "
			}

			// Falls !ok > 0 ist, den Counter gelb markieren
			if failCount > 0 {
				counterFormat = fmt.Sprintf("\033[93m%s\033[0m", counterFormat)
			}

			// Einheitliche Formatierung mit fester Spaltenbreite
			output.WriteString(fmt.Sprintf("%-18s  %s%-8s\033[0m  %s  %s  %s up",
				ip, statusColor, status, counterFormat, latencyStr, uptimeStr))

			// Zeige Status-Änderung
			if !statusChangeTime.IsZero() {
				timeSince := time.Since(statusChangeTime)
				var changeLabel string
				if isOnline {
					changeLabel = "down"
				} else {
					changeLabel = "up"
				}
				output.WriteString(fmt.Sprintf("  (%s: %s)", changeLabel, formatTimeSince(timeSince)))
			}
			output.WriteString("\n")
		}

		clearConsole()
		fmt.Print(output.String())
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
