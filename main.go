package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/schollz/wifiscan"
	speedtest "github.com/showwin/speedtest-go/speedtest"
)

func checkIsRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	return currentUser.Uid == "0" // Root has UID 0
}

// WiFiNetwork represents a single WiFi network with its properties
type WiFiNetwork struct {
	SSID         string
	BSSID        string
	Signal       int
	SignalDBm    int
	Quality      string
	QualityLabel string
	InUse        bool
}

// scanWithNmcli attempts to scan WiFi networks using the nmcli command
func scanWithNmcli() error {
	// Check if nmcli is available
	_, err := exec.LookPath("nmcli")
	if err != nil {
		return fmt.Errorf("nmcli not found: %v", err)
	}

	// Use a channel to receive processed network data
	networkChan := make(chan WiFiNetwork)
	errorChan := make(chan error)

	// Process nmcli output in a goroutine
	go func() {
		// Try a simpler approach with standard output format
		cmd := exec.Command("nmcli", "-c", "no", "device", "wifi", "list", "--rescan", "yes")
		output, err := cmd.CombinedOutput()
		if err != nil {
			errorChan <- fmt.Errorf("nmcli failed: %v\n%s", err, output)
			return
		}

		if len(output) == 0 {
			errorChan <- fmt.Errorf("no output from nmcli command")
			return
		}

		// Parse the output line by line
		lines := strings.Split(string(output), "\n")
		if len(lines) <= 1 {
			// Try an alternative method
			shellCmd := exec.Command("sh", "-c", "sudo nmcli device wifi list")
			shellOutput, shellErr := shellCmd.CombinedOutput()
			if shellErr != nil {
				errorChan <- fmt.Errorf("no networks found or command didn't return proper output")
				return
			}
			lines = strings.Split(string(shellOutput), "\n")
			if len(lines) <= 1 {
				errorChan <- fmt.Errorf("no networks found with alternative command")
				return
			}
		}

		// Process each line in parallel with goroutines
		var wg sync.WaitGroup

		// Process each line (skip header)
		for i, line := range lines {
			if i == 0 || line == "" {
				continue // Skip header and empty lines
			}

			wg.Add(1)
			go func(line string) {
				defer wg.Done()

				// Parse the line
				network, err := parseNetworkLine(line)
				if err != nil {
					// Just skip problematic lines
					return
				}

				// Send the network to the main goroutine
				networkChan <- network
			}(line)
		}

		// Wait for all parsing goroutines to finish
		go func() {
			wg.Wait()
			close(networkChan)
		}()
	}()

	// Wait a bit to ensure we show the header first
	time.Sleep(100 * time.Millisecond)

	// Display the header
	fmt.Println("Available Wi-Fi Networks:")
	fmt.Println("-------------------------")
	fmt.Printf("%-30s %-20s %-20s %-15s\n", "SSID", "MAC Address", "Signal Strength", "Quality")
	fmt.Println("-------------------------------------------------------------------------")

	// Collect the networks
	var networks []WiFiNetwork

	// Process results from the channel
	for {
		select {
		case err := <-errorChan:
			return err
		case network, ok := <-networkChan:
			if !ok {
				// Channel closed, all networks processed
				// Sort networks by signal strength (strongest first)
				sort.Slice(networks, func(i, j int) bool {
					return networks[i].Signal > networks[j].Signal
				})

				// Print the networks
				for _, network := range networks {
					fmt.Printf("%-30s %-20s %-20s %-15s\n",
						network.SSID,
						network.BSSID,
						fmt.Sprintf("%d%% (%d dBm)", network.Signal, network.SignalDBm),
						fmt.Sprintf("%s (%s)", network.Quality, network.QualityLabel))
				}

				if len(networks) == 0 {
					fmt.Println("No WiFi networks found. Make sure your WiFi adapter is enabled.")
				}

				return nil
			}
			networks = append(networks, network)
		}
	}

	// This code is unreachable because the function always returns
	// from within the select statement above when the channel is closed
	// or an error occurs
}

// parseNetworkLine parses a single line from nmcli output into a WiFiNetwork struct
func parseNetworkLine(line string) (WiFiNetwork, error) {
	network := WiFiNetwork{}

	// Split the line by whitespace
	fields := strings.Fields(line)
	if len(fields) < 7 { // Minimum fields we expect
		return network, fmt.Errorf("not enough fields in line")
	}

	// Check if this network is in use (has * at the start)
	startIdx := 0
	if fields[0] == "*" {
		network.InUse = true
		startIdx = 1
	}

	// Get the BSSID (MAC address)
	network.BSSID = fields[startIdx]

	// SSID might contain spaces, so we need to handle that
	// Format: [IN-USE] BSSID SSID MODE CHAN RATE SIGNAL BARS SECURITY

	// Find the index of "Infra" which marks the end of SSID
	infraIndex := -1
	for i, field := range fields[startIdx+1:] {
		if field == "Infra" {
			infraIndex = startIdx + 1 + i
			break
		}
	}

	// If we found "Infra", use that as the SSID end, otherwise use the original calculation
	if infraIndex != -1 {
		network.SSID = strings.Join(fields[startIdx+1:infraIndex], " ")
	} else {
		// Fallback to original calculation
		ssidEnd := len(fields) - 6 // Last 6 fields are: MODE, CHAN, RATE, SIGNAL, BARS, SECURITY
		network.SSID = strings.Join(fields[startIdx+1:ssidEnd], " ")
	}

	// Handle hidden networks
	if network.SSID == "--" {
		network.SSID = "[Hidden Network]"
	}

	// Signal strength is 2nd from the end before BARS and SECURITY
	signalStr := fields[len(fields)-3]
	signal, err := strconv.Atoi(signalStr)
	if err != nil {
		return network, fmt.Errorf("could not parse signal strength: %v", err)
	}

	// Set signal and convert to dBm
	network.Signal = signal
	network.SignalDBm = -100 + (signal * 60 / 100) // 0% ~ -100 dBm, 100% ~ -40 dBm

	// Get the quality bars
	network.Quality = fields[len(fields)-2]
	network.QualityLabel = getSignalQualityLabelFromPercent(signal)

	return network, nil
}

// getSignalQualityLabelFromPercent returns quality label based on percentage (0-100)
func getSignalQualityLabelFromPercent(percent int) string {
	if percent >= 80 {
		return "Excellent"
	} else if percent >= 60 {
		return "Good"
	} else if percent >= 40 {
		return "Fair"
	} else if percent >= 20 {
		return "Poor"
	}
	return "Very Poor"
}

func scanWiFi() {
	// Check if running as root
	if !checkIsRoot() {
		fmt.Println("❌ Error: WiFi scanning requires root privileges.")
		fmt.Println("Please run the command with sudo: sudo ./wifi-speed-cli scan")
		return
	}

	fmt.Println("Scanning for WiFi networks...")

	// First attempt: try nmcli (more reliable for showing actual SSIDs)
	err := scanWithNmcli()
	if err != nil {
		fmt.Printf("Error with primary scanning method (nmcli): %v\n", err)
		fmt.Println("Trying alternative scanning method...")

		// Second attempt: try wifiscan library
		networks, err := wifiscan.Scan()
		if err != nil {
			fmt.Printf("Error with alternative scanning method: %v\n", err)
			fmt.Println("\nPossible causes:")
			fmt.Println("- WiFi adapter might be disabled")
			fmt.Println("- Required dependencies might be missing (try: sudo apt install network-manager)")
			fmt.Println("- Permission issues with network interfaces")
			fmt.Println("\nTroubleshooting:")
			fmt.Println("1. Ensure WiFi is enabled: rfkill unblock wifi")
			fmt.Println("2. Check if NetworkManager is running: systemctl status NetworkManager")
			fmt.Println("3. Check available WiFi adapters: ip link show")
			return
		}

		// Sort networks by signal strength (strongest first)
		sort.Slice(networks, func(i, j int) bool {
			return networks[i].RSSI > networks[j].RSSI
		})

		fmt.Println("Available Wi-Fi Networks:")
		fmt.Println("-------------------------")
		fmt.Printf("%-30s %-20s %-20s %-15s\n", "SSID", "MAC Address", "Signal Strength", "Quality")
		fmt.Println("-------------------------------------------------------------------------")

		uniqueNetworks := make(map[string]bool)
		for _, network := range networks {
			// Skip duplicate entries
			if uniqueNetworks[network.SSID] {
				continue
			}
			uniqueNetworks[network.SSID] = true

			// Determine SSID and MAC address
			ssid := network.SSID
			macAddress := ""

			// Check if it looks like a MAC address (contains ":" or has hexadecimal format)
			if strings.Contains(ssid, ":") || isLikelyMacAddress(ssid) {
				// This is likely a MAC address for a hidden network
				macAddress = ssid
				ssid = "[Hidden Network]"
			} else {
				// For networks with proper SSIDs, we don't have MAC address from the wifiscan library
				// The wifiscan library doesn't provide MAC addresses directly in its API
				macAddress = "N/A" // Not available in this scan method
			}

			// Calculate signal quality percentage (RSSI typically ranges from -100 to 0)
			qualityPercentage := 0
			if network.RSSI >= -30 {
				qualityPercentage = 100
			} else if network.RSSI <= -100 {
				qualityPercentage = 0
			} else {
				qualityPercentage = 100 - (int(float64(network.RSSI+30) / -70.0 * 100.0))
			}

			fmt.Printf("%-30s %-20s %-20s %-15s\n",
				ssid,
				macAddress,
				fmt.Sprintf("%d dBm", network.RSSI),
				fmt.Sprintf("%d%% (%s)", qualityPercentage, getSignalQualityLabel(qualityPercentage)))
		}

		if len(networks) == 0 {
			fmt.Println("No WiFi networks found. Make sure your WiFi adapter is enabled.")
		}
	}
}

// isLikelyMacAddress checks if the string looks like a MAC address without colons
func isLikelyMacAddress(s string) bool {
	// Common MAC address patterns without colons
	return len(s) == 12 && isHexString(s)
}

// isHexString checks if a string is composed of hexadecimal characters
func isHexString(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// getSignalQualityLabel returns a human-readable label for the signal quality
func getSignalQualityLabel(quality int) string {
	if quality >= 80 {
		return "Excellent"
	} else if quality >= 60 {
		return "Good"
	} else if quality >= 40 {
		return "Fair"
	} else if quality >= 20 {
		return "Poor"
	}
	return "Very Poor"
}

func testSpeed() {
	fmt.Println("\nTesting network speed...")
	_, err := speedtest.FetchUserInfo()
	if err != nil {
		log.Fatalf("Error fetching user info: %v", err)
	}

	serverList, err := speedtest.FetchServers()
	if err != nil {
		log.Fatalf("Error fetching servers: %v", err)
	}

	if len(serverList) == 0 {
		log.Fatalf("No servers available for testing")
	}

	targetServer := serverList[0]
	fmt.Println("Selected Server:", targetServer.Name, "(", targetServer.Country, ")")
	fmt.Println("Server Sponsor:", targetServer.Sponsor)
	fmt.Println("Distance:", targetServer.Distance, "km")
	fmt.Println("\nRunning tests...")

	err = targetServer.DownloadTest()
	if err != nil {
		log.Fatalf("Error during download test: %v", err)
	}

	err = targetServer.UploadTest()
	if err != nil {
		log.Fatalf("Error during upload test: %v", err)
	}

	// Apply scaling factor if needed (if values are unreasonably high)
	dlSpeed := float64(targetServer.DLSpeed)
	ulSpeed := float64(targetServer.ULSpeed)

	// Check if speeds are unreasonably high (> 10 Gbps) and scale down
	// This is a workaround for the potential unit conversion issue
	if dlSpeed > 10000 {
		dlSpeed /= 1000000 // Scale from bits to Mbps if needed
	}
	if ulSpeed > 10000 {
		ulSpeed /= 1000000 // Scale from bits to Mbps if needed
	}

	fmt.Printf("\nDownload Speed: %.2f Mbps\n", dlSpeed)
	fmt.Printf("Upload Speed: %.2f Mbps\n", ulSpeed)

	// Convert latency from time.Duration to float64 milliseconds
	latencyMs := float64(targetServer.Latency) / float64(time.Millisecond)
	fmt.Printf("Ping (Latency): %.2f ms\n", latencyMs)

	// Print a summary of the connection quality
	printConnectionQuality(dlSpeed, ulSpeed, latencyMs)
}

// printConnectionQuality provides a human-readable assessment of the connection quality
func printConnectionQuality(downloadSpeed, uploadSpeed, latency float64) {
	fmt.Println("\nConnection Quality Assessment:")
	fmt.Println("-----------------------------")

	// Download assessment
	fmt.Printf("Download: ")
	if downloadSpeed >= 100 {
		fmt.Println("Excellent (4K streaming, large downloads)")
	} else if downloadSpeed >= 25 {
		fmt.Println("Good (HD streaming, video calls)")
	} else if downloadSpeed >= 5 {
		fmt.Println("Fair (SD streaming, web browsing)")
	} else {
		fmt.Println("Poor (basic web browsing)")
	}

	// Upload assessment
	fmt.Printf("Upload: ")
	if uploadSpeed >= 20 {
		fmt.Println("Excellent (video uploads, live streaming)")
	} else if uploadSpeed >= 5 {
		fmt.Println("Good (video calls, file uploads)")
	} else if uploadSpeed >= 1 {
		fmt.Println("Fair (photo uploads, email)")
	} else {
		fmt.Println("Poor (basic web tasks)")
	}

	// Latency assessment
	fmt.Printf("Latency: ")
	if latency < 20 {
		fmt.Println("Excellent (competitive gaming)")
	} else if latency < 50 {
		fmt.Println("Good (online gaming, video calls)")
	} else if latency < 100 {
		fmt.Println("Fair (web browsing, streaming)")
	} else {
		fmt.Println("Poor (may experience lag)")
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./wifi-speed-cli [scan | speedtest]")
		return
	}

	switch os.Args[1] {
	case "scan":
		scanWiFi()
	case "speedtest":
		testSpeed()
	default:
		fmt.Println("Invalid command. Use 'scan' to list Wi-Fi networks or 'speedtest' to check network speed.")
	}
}
