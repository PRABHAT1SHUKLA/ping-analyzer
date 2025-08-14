package main

import (
	"bufio"
	"flag"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const (
	defaultTarget        = "google.com" // Default ping target
	defaultCount         = 10           // Default number of pings (0 for infinite)
	defaultInterval      = 1            // Seconds between pings
	highLatencyThreshold = 100          // ms; alert if above this
)

func main() {
	target := flag.String("target", defaultTarget, "Target host to ping (e.g., google.com)")
	count := flag.Int("count", defaultCount, "Number of pings (0 for infinite)")
	interval := flag.Int("interval", defaultInterval, "Interval between pings in seconds")
	logFile := flag.String("log", "ping_log.txt", "File to log results")

	flag.Parse()

}

// pingOnce runs a single ping and returns the latency in ms or -1 on error
func pingOnce(target string) float64 {
	cmd := exec.Command("ping", "-c", "1", target) // -c 1 means one ping
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error pinging %s: %v\n", target, err)
		return -1
	}

	// Parse output for time, e.g., "time=10.2 ms"
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "time=") {
			parts := strings.Split(line, "time=")
			if len(parts) > 1 {
				msStr := strings.Split(parts[1], " ")[0] // Get "10.2"
				ms, err := strconv.ParseFloat(msStr, 64)
				if err == nil {
					return ms
				}
			}
		}
	}
	return -1 // No latency found
}
