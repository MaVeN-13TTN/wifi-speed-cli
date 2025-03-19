# WiFi Speed CLI

A command-line tool to scan WiFi networks and test internet connection speed with human-readable output.

## Overview

WiFi Speed CLI is a simple yet powerful command-line utility written in Go that provides two main functionalities:

1. Scanning available WiFi networks and displaying their SSID/MAC, signal strength, and quality assessment
2. Testing your internet connection speed (download, upload, and latency) with a detailed quality assessment

## Installation

### Prerequisites

- Go 1.22 or later
- Linux operating system
- NetworkManager (for fallback WiFi scanning if primary method fails)

### Build from Source

1. Clone the repository:

   ```bash
   git clone https://github.com/yourusername/wifi-speed-cli.git
   cd wifi-speed-cli
   ```

2. Build the application:

   ```bash
   go build -o wifi-speed-cli
   ```

3. (Optional) Move the binary to a location in your PATH:
   ```bash
   sudo mv wifi-speed-cli /usr/local/bin/
   ```

## Usage

WiFi Speed CLI supports two main commands:

### Scan WiFi Networks

⚠️ **Important**: WiFi scanning requires root privileges. You must use `sudo` when running this command:

```bash
sudo ./wifi-speed-cli scan
```

Example output:

```
Scanning for WiFi networks...
Available Wi-Fi Networks:
-------------------------
SSID/MAC                        Signal Strength       Quality
-------------------------------------------------------------------------
MyNetwork                        80% (-58 dBm)         ▂▄▆█ (Excellent)
[Hidden Network]                 70% (-64 dBm)         ▂▄▆_ (Good)
Neighbor's WiFi                  50% (-76 dBm)         ▂▄__ (Fair)
```

#### Understanding the WiFi Scan Output

- **SSID/MAC**: The name of the WiFi network or labeled as [Hidden Network] if SSID is not broadcast
- **Signal Strength**: Shown in both percentage and dBm (decibel-milliwatts)
  - Excellent: -30 to -50 dBm
  - Good: -50 to -65 dBm
  - Fair: -65 to -75 dBm
  - Poor: Below -75 dBm
- **Quality**: Visual signal bars and qualitative assessment
  - 80-100%: Excellent (▂▄▆█)
  - 60-80%: Good (▂▄▆\_)
  - 40-60%: Fair (▂▄\_\_)
  - 20-40%: Poor (▂\_\_\_)
  - 0-20%: Very Poor (\_\_\_)

### Test Internet Speed

To test your current internet connection speed with detailed metrics (does not require root privileges):

```bash
./wifi-speed-cli speedtest
```

Example output:

```
Testing network speed...
Selected Server: Nairobi (Kenya)
Server Sponsor: Zuku
Distance: 0.16 km

Running tests...

Download Speed: 14.43 Mbps
Upload Speed: 13.91 Mbps
Ping (Latency): 15.90 ms

Connection Quality Assessment:
-----------------------------
Download: Fair (SD streaming, web browsing)
Upload: Good (video calls, file uploads)
Latency: Excellent (competitive gaming)
```

## How It Works

### WiFi Scanning

The application uses two methods to scan for WiFi networks:

1. **Primary Method**: Uses the `github.com/schollz/wifiscan` library to detect and analyze nearby WiFi networks
2. **Fallback Method**: If the primary method fails, the application uses `nmcli` (NetworkManager command-line tool) to scan for networks

This dual approach ensures greater compatibility across different Linux distributions and hardware configurations.

### Speed Testing

For internet speed testing, the application uses the `github.com/showwin/speedtest-go` library, which provides functionality similar to speedtest.net. The tool:

1. Fetches user and server information
2. Selects a test server (usually the closest one)
3. Performs download, upload, and latency tests
4. Provides a quality assessment for each metric

## Dependencies

- [github.com/schollz/wifiscan](https://github.com/schollz/wifiscan) - Primary WiFi network scanning
- [github.com/showwin/speedtest-go](https://github.com/showwin/speedtest-go) - For internet speed testing
- NetworkManager (`nmcli`) - Fallback WiFi scanning method

## Troubleshooting

### WiFi Scanning Issues

- **Permission denied**: Run the application with sudo (`sudo ./wifi-speed-cli scan`)
- **Scan fails with both methods**:
  - Ensure your WiFi adapter is enabled: `rfkill unblock wifi`
  - Check if NetworkManager is running: `systemctl status NetworkManager`
  - Verify available WiFi adapters: `ip link show`
  - Install NetworkManager if needed: `sudo apt install network-manager`
- **Hidden networks**: Networks that don't broadcast their SSID will be shown as [Hidden Network]

### Speed Test Issues

- **Unrealistic speeds**: If you see extremely high speeds (like multiple Gbps), there might be a unit conversion issue. The application attempts to correct this automatically
- **Test fails**: Ensure you have an active internet connection
- **Slow speeds**: Try running the test at different times of day, as network congestion can affect results

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Acknowledgements

- [speedtest.net](https://www.speedtest.net/) for providing the speed test infrastructure
- All contributors of the libraries used in this project
