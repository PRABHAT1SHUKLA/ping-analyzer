package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/guptarohit/asciigraph"
)

const (
	defaultTarget        = "google.com" // Default ping target
	defaultCount         = 10           // Default number of pings (0 for infinite)
	defaultInterval      = 1            // Seconds between pings
	highLatencyThreshold = 100.0        // ms; alert if above this
)

type PingResult struct {
	sequence  int
	latency   float64
	success   bool
	timestamp time.Time
}

func main() {
	target := flag.String("target", defaultTarget, "Target host to ping (e.g., google.com)")
	count := flag.Int("count", defaultCount, "Number of pings (0 for infinite)")
	interval := flag.Int("interval", defaultInterval, "Interval between pings in seconds")
	logFile := flag.String("log", "ping_log.txt", "File to log results")
	threshold := flag.Float64("threshold", highLatencyThreshold, "High latency threshold in ms")

	flag.Parse()

	if *interval < 1 {
		fmt.Println("Error: Interval must be at least 1 second")
		os.Exit(1)
	}

	latencies := []float64{}
	results := []PingResult{}

	fmt.Printf("Pinging %s every %d seconds", *target, *interval)
	if *count > 0 {
		fmt.Printf(" (%d times)...\n", *count)
	} else {
		fmt.Printf(" (infinite - press Ctrl+C to stop)...\n")
	}

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	defer ticker.Stop()

	done := make(chan bool)

	// Goroutine for continuous pinging
	go func() {
		defer close(done)
		i := 0
		for {
			select {
			case <-sigChan:
				fmt.Println("\nReceived interrupt signal. Stopping...")
				return
			case <-ticker.C:
				result := pingOnce(*target, i+1)
				results = append(results, result)

				if result.success {
					latencies = append(latencies, result.latency)
					fmt.Printf("Ping %d: %.2f ms", result.sequence, result.latency)
					if result.latency > *threshold {
						fmt.Printf(" [HIGH LATENCY ALERT: %.2f ms > %.0f ms]", result.latency, *threshold)
					}
					fmt.Println()
				} else {
					fmt.Printf("Ping %d: Request timeout or host unreachable\n", result.sequence)
				}

				i++
				if *count > 0 && i >= *count {
					return
				}
			}
		}
	}()

	// Wait for completion
	<-done

	// Display statistics
	displayStats(results, latencies)

	// Generate and display graph
	if len(latencies) > 0 {
		fmt.Println("\nLatency Graph:")
		graph := asciigraph.Plot(latencies,
			asciigraph.Height(10),
			asciigraph.Caption(fmt.Sprintf("Latency over time (ms) - Target: %s", *target)),
			asciigraph.Width(60))
		fmt.Println(graph)
	} else {
		fmt.Println("No successful pings to display graph.")
	}

	// Log results
	if err := logResults(*logFile, results, *target); err != nil {
		fmt.Printf("Error logging results: %v\n", err)
	} else {
		fmt.Printf("Results logged to %s\n", *logFile)
	}
}

// pingOnce runs a single ping and returns the result
func pingOnce(target string, sequence int) PingResult {
	result := PingResult{
		sequence:  sequence,
		timestamp: time.Now(),
		success:   false,
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", target)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "5000", target) // 5 second timeout
	}

	output, err := cmd.Output()
	if err != nil {
		return result
	}

	// Parse output for latency
	latency := parseLatency(string(output))
	if latency >= 0 {
		result.latency = latency
		result.success = true
	}

	return result
}

// parseLatency extracts latency from ping output
func parseLatency(output string) float64 {
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		// Handle different ping output formats
		if runtime.GOOS == "windows" {
			// Windows format: "time<1ms" or "time=10ms"
			if strings.Contains(line, "time") && strings.Contains(line, "ms") {
				if strings.Contains(line, "time<") {
					parts := strings.Split(line, "time<")
					if len(parts) > 1 {
						msStr := strings.Split(parts[1], "ms")[0]
						if ms, err := strconv.ParseFloat(msStr, 64); err == nil {
							return ms - 0.5 // Assume <1ms means ~0.5ms
						}
					}
				} else if strings.Contains(line, "time=") {
					parts := strings.Split(line, "time=")
					if len(parts) > 1 {
						msStr := strings.Split(parts[1], "ms")[0]
						if ms, err := strconv.ParseFloat(msStr, 64); err == nil {
							return ms
						}
					}
				}
			}
		} else {
			// Unix/Linux format: "time=10.2 ms"
			if strings.Contains(line, "time=") {
				parts := strings.Split(line, "time=")
				if len(parts) > 1 {
					msStr := strings.Fields(parts[1])[0] // Get first field after "time="
					if ms, err := strconv.ParseFloat(msStr, 64); err == nil {
						return ms
					}
				}
			}
		}
	}

	return -1 // No latency found
}

// displayStats shows ping statistics
func displayStats(results []PingResult, latencies []float64) {
	if len(results) == 0 {
		return
	}

	successful := len(latencies)
	total := len(results)
	packetLoss := float64(total-successful) / float64(total) * 100

	fmt.Println("\n--- Ping Statistics ---")
	fmt.Printf("Packets sent: %d\n", total)
	fmt.Printf("Packets received: %d\n", successful)
	fmt.Printf("Packet loss: %.1f%%\n", packetLoss)

	if successful > 0 {
		min, max, avg := calculateStats(latencies)
		fmt.Printf("Latency - Min: %.2f ms, Max: %.2f ms, Avg: %.2f ms\n", min, max, avg)
	}
}

// calculateStats computes min, max, and average latency
func calculateStats(latencies []float64) (min, max, avg float64) {
	if len(latencies) == 0 {
		return 0, 0, 0
	}

	min = latencies[0]
	max = latencies[0]
	sum := 0.0

	for _, latency := range latencies {
		if latency < min {
			min = latency
		}
		if latency > max {
			max = latency
		}
		sum += latency
	}

	avg = sum / float64(len(latencies))
	return min, max, avg
}

// logResults writes results to a file
func logResults(filename string, results []PingResult, target string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.WriteString(fmt.Sprintf("Ping Log - Target: %s\n", target))
	writer.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	writer.WriteString("=====================================\n")

	// Write results
	for _, result := range results {
		timestamp := result.timestamp.Format("15:04:05")
		if result.success {
			writer.WriteString(fmt.Sprintf("[%s] Ping %d: %.2f ms\n",
				timestamp, result.sequence, result.latency))
		} else {
			writer.WriteString(fmt.Sprintf("[%s] Ping %d: FAILED\n",
				timestamp, result.sequence))
		}
	}

	return nil
}
