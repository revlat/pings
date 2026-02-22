package pinger

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// HostState holds the monitoring state for a single host.
type HostState struct {
	IP               string
	Online           bool
	OkCount          int
	FailCount        int
	Latency          time.Duration
	LastSuccess      time.Time
	PreviousStatus   *bool
	LastStatusChange time.Time
}

// Monitor manages the ping loop for multiple hosts.
type Monitor struct {
	mu     sync.Mutex
	states map[string]*HostState
}

// NewMonitor creates a new Monitor instance.
func NewMonitor() *Monitor {
	return &Monitor{
		states: make(map[string]*HostState),
	}
}

// Start begins the ping monitoring loop. Blocks until the process exits.
// The callback is called after each full scan cycle with a snapshot of all host states
// in the original order of ips.
func (m *Monitor) Start(ips []string, interval int, callback func([]HostState)) {
	m.mu.Lock()
	for _, ip := range ips {
		m.states[ip] = &HostState{IP: ip}
	}
	m.mu.Unlock()

	type pingResult struct {
		ip      string
		success bool
		latency time.Duration
	}

	for {
		results := make(chan pingResult, len(ips))
		var wg sync.WaitGroup

		for _, ip := range ips {
			wg.Add(1)
			go func(host string) {
				defer wg.Done()
				success, latency := PingHost(host)
				results <- pingResult{host, success, latency}
			}(ip)
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		for result := range results {
			m.mu.Lock()
			state := m.states[result.ip]

			currentStatus := result.success
			if state.PreviousStatus != nil && *state.PreviousStatus != currentStatus {
				state.LastStatusChange = time.Now()
			}

			if result.success {
				state.OkCount++
				state.Latency = result.latency
				state.LastSuccess = time.Now()
				state.Online = true
			} else {
				state.FailCount++
				state.Online = false
			}

			statusCopy := currentStatus
			state.PreviousStatus = &statusCopy
			m.mu.Unlock()
		}

		// Build ordered snapshot for callback.
		m.mu.Lock()
		snapshot := make([]HostState, len(ips))
		for i, ip := range ips {
			snapshot[i] = *m.states[ip]
		}
		m.mu.Unlock()

		callback(snapshot)

		time.Sleep(time.Duration(interval) * time.Second)
	}
}

// PingHost executes a single ping to an IP and returns success and latency.
func PingHost(ip string) (bool, time.Duration) {
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
		if err != nil {
			return false, 0
		}
		output := out.String()
		hasReply := strings.Contains(output, "Antwort von") ||
			strings.Contains(output, "Reply from")
		hasError := strings.Contains(output, "nicht erreichbar") ||
			strings.Contains(output, "Zeitüberschreitung") ||
			strings.Contains(output, "unreachable") ||
			strings.Contains(output, "timed out")
		if hasReply && !hasError {
			return true, latency
		}
		return false, 0
	}

	if err == nil {
		return true, latency
	}
	return false, 0
}

// LoadIPsFromFile loads IP addresses from a file, one per line.
func LoadIPsFromFile(filename string) ([]string, error) {
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

// FormatTimeSince formats a duration into a human-readable "Xh Xm Xs ago" string.
func FormatTimeSince(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds ago", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds ago", minutes, seconds)
	}
	return fmt.Sprintf("%ds ago", seconds)
}
