package parser

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type RegistryParser struct {
	InterfaceNames map[string]string // GUID -> interface name
	InterfaceIPs   map[string]IPInfo // GUID -> IP info
}

type IPInfo struct {
	DHCPIP   string
	StaticIP string
}

type Mapping struct {
	IP            string
	InterfaceName string
}

func NewRegistryParser() *RegistryParser {
	return &RegistryParser{
		InterfaceNames: make(map[string]string),
		InterfaceIPs:   make(map[string]IPInfo),
	}
}

func (rp *RegistryParser) ParseNetworkFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Warning: Network file %s not found\n", filename)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentGUID string

	guidRegex := regexp.MustCompile(`\[HKEY_LOCAL_MACHINE\\SYSTEM\\ControlSet001\\Control\\Network\\{4D36E972-E325-11CE-BFC1-08002BE10318}\\{([^}]+)}\]`)
	nameRegex := regexp.MustCompile(`"Name"="([^"]+)"`)

	for scanner.Scan() {
		line := scanner.Text()

		// Look for interface GUID sections
		if matches := guidRegex.FindStringSubmatch(line); matches != nil {
			currentGUID = matches[1]
			continue
		}

		// Look for Connection subsection with Name
		if currentGUID != "" && strings.Contains(line, `"Name"`) {
			if matches := nameRegex.FindStringSubmatch(line); matches != nil {
				name := matches[1]
				rp.InterfaceNames[currentGUID] = name
				currentGUID = ""
			}
		}
	}

	return scanner.Err()
}

func (rp *RegistryParser) ParseServiceFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Warning: Service file %s not found\n", filename)
		return nil
	}
	defer file.Close()

	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Split content into lines and parse manually
	lines := strings.Split(string(content), "\n")

	var currentGUID string
	var sectionContent strings.Builder

	guidRegex := regexp.MustCompile(`\[HKEY_LOCAL_MACHINE\\SYSTEM\\ControlSet001\\Services\\Tcpip\\Parameters\\Interfaces\\{([^}]+)}\]`)
	dhcpIPRegex := regexp.MustCompile(`"DhcpIPAddress"="([^"]+)"`)
	staticIPRegex := regexp.MustCompile(`"IPAddress"=hex\(7\):([^,\n]+)`)
	dhcpEnabledRegex := regexp.MustCompile(`"EnableDHCP"=dword:00000001`)

	for _, line := range lines {
		// Check for new interface section
		if matches := guidRegex.FindStringSubmatch(line); matches != nil {
			// Process previous section if exists
			if currentGUID != "" {
				rp.processInterfaceSection(currentGUID, sectionContent.String(), dhcpIPRegex, staticIPRegex, dhcpEnabledRegex)
			}

			currentGUID = matches[1]
			sectionContent.Reset()
			continue
		}

		// Add line to current section content
		if currentGUID != "" {
			if sectionContent.Len() > 0 {
				sectionContent.WriteString("\n")
			}
			sectionContent.WriteString(line)
		}
	}

	// Process last section
	if currentGUID != "" {
		rp.processInterfaceSection(currentGUID, sectionContent.String(), dhcpIPRegex, staticIPRegex, dhcpEnabledRegex)
	}

	return nil
}

func (rp *RegistryParser) processInterfaceSection(guid, sectionContent string, dhcpIPRegex, staticIPRegex, dhcpEnabledRegex *regexp.Regexp) {
	ipInfo := IPInfo{}

	// Check if DHCP is enabled and extract DHCP IP
	if dhcpEnabledRegex.MatchString(sectionContent) {
		if matches := dhcpIPRegex.FindStringSubmatch(sectionContent); matches != nil {
			ipInfo.DHCPIP = matches[1]
		}
	}

	// Extract static IP if present
	if matches := staticIPRegex.FindStringSubmatch(sectionContent); matches != nil {
		hexIP := strings.TrimSpace(matches[1])
		hexIP = strings.ReplaceAll(hexIP, ",", "")
		hexIP = strings.ReplaceAll(hexIP, " ", "")

		if ipString := rp.hexToString(hexIP); ipString != "" && strings.Contains(ipString, ".") {
			ipInfo.StaticIP = ipString
		}
	}

	rp.InterfaceIPs[guid] = ipInfo
}

func (rp *RegistryParser) hexToString(hexString string) string {
	// Remove any whitespace
	hexString = strings.ReplaceAll(hexString, " ", "")

	// Convert hex to bytes
	bytes, err := hex.DecodeString(hexString)
	if err != nil {
		return ""
	}

	// Convert UTF-16LE to string
	if len(bytes) < 2 {
		return ""
	}

	// UTF-16LE: every two bytes represent one character
	var result strings.Builder
	for i := 0; i < len(bytes)-1; i += 2 {
		if bytes[i] == 0 && bytes[i+1] == 0 {
			break // Null terminator
		}
		result.WriteByte(bytes[i+1]) // High byte first in UTF-16LE
		result.WriteByte(bytes[i])   // Low byte second
	}

	return result.String()
}

func (rp *RegistryParser) GenerateMapping() []Mapping {
	var mappings []Mapping

	for guid, ipInfo := range rp.InterfaceIPs {
		// Determine the primary IP (prefer DHCP over static)
		primaryIP := ipInfo.DHCPIP
		if primaryIP == "" {
			primaryIP = ipInfo.StaticIP
		}

		if primaryIP != "" {
			// Try case-insensitive GUID matching
			interfaceName := fmt.Sprintf("Unknown_%s", guid[:8])
			for nameGUID, name := range rp.InterfaceNames {
				if strings.EqualFold(nameGUID, guid) {
					interfaceName = name
					break
				}
			}

			mapping := Mapping{
				IP:            primaryIP,
				InterfaceName: interfaceName,
			}
			mappings = append(mappings, mapping)
		}
	}

	return mappings
}

func (rp *RegistryParser) PrintMappingTable(mappings []Mapping) string {
	var result strings.Builder

	if len(mappings) == 0 {
		result.WriteString("No IP-to-interface mappings found.\n")
		return result.String()
	}

	result.WriteString("IP-to-Interface Name Mapping:\n")
	result.WriteString("========================================")
	result.WriteString("\n")
	result.WriteString(fmt.Sprintf("%-15s %-20s\n", "IP Address", "Interface Name"))
	result.WriteString("----------------------------------------\n")

	for _, mapping := range mappings {
		result.WriteString(fmt.Sprintf("%-15s %-20s\n", mapping.IP, mapping.InterfaceName))
	}

	return result.String()
}

func (rp *RegistryParser) PrintCSVOutput(mappings []Mapping) {
	if len(mappings) == 0 {
		return
	}

	fmt.Println("\nCSV Output:")
	fmt.Println("ip,interface_name")
	for _, mapping := range mappings {
		fmt.Printf("%s,%s\n", mapping.IP, mapping.InterfaceName)
	}
}

func (rp *RegistryParser) DebugInfo() {
	fmt.Println("\nDebug Information:")
	fmt.Println("==================================================")
	fmt.Printf("Found %d interface names:\n", len(rp.InterfaceNames))
	for guid, name := range rp.InterfaceNames {
		fmt.Printf("  %s -> %s\n", guid, name)
	}
	fmt.Printf("\nFound %d IP configurations:\n", len(rp.InterfaceIPs))
	for guid, ipInfo := range rp.InterfaceIPs {
		ip := ipInfo.DHCPIP
		if ip == "" {
			ip = ipInfo.StaticIP
		}
		fmt.Printf("  %s -> %s\n", guid, ip)
	}
}

// parser := NewRegistryParser()

// // Parse files
// parser.ParseServiceFile(serviceFile)
// parser.ParseNetworkFile(networkFile)

// // Generate mappings
// mappings := parser.GenerateMapping()
