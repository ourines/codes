package httpserver

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// startMDNS registers a Bonjour/mDNS service so iOS clients can discover
// this HTTP server on the local network via _codes._tcp.
//
// Instead of using a Go mDNS library (which conflicts with macOS
// mDNSResponder on UDP 5353), we delegate to the system's dns-sd command
// (macOS) or avahi-publish-service (Linux).
// Returns a shutdown function that kills the registration process.
func startMDNS(port int, version string) func() {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "codes-server"
	}

	// macOS: dns-sd -R delegates to mDNSResponder (no port conflicts)
	if path, err := exec.LookPath("dns-sd"); err == nil {
		return registerViaDNSSD(path, hostname, port, version)
	}

	// Linux: avahi-publish-service
	if path, err := exec.LookPath("avahi-publish-service"); err == nil {
		return registerViaAvahi(path, hostname, port, version)
	}

	log.Printf("[mDNS] No dns-sd or avahi-publish-service found; skipping mDNS registration")
	return func() {}
}

func registerViaDNSSD(path, hostname string, port int, version string) func() {
	cmd := exec.Command(path, "-R", hostname,
		"_codes._tcp", "local",
		strconv.Itoa(port),
		fmt.Sprintf("port=%d", port),
		fmt.Sprintf("version=%s", version),
		fmt.Sprintf("host=%s.local", hostname),
	)
	if err := cmd.Start(); err != nil {
		log.Printf("[mDNS] Failed to start dns-sd: %v", err)
		return func() {}
	}
	log.Printf("[mDNS] Registered '%s._codes._tcp.local' on port %d (dns-sd pid %d)", hostname, port, cmd.Process.Pid)
	return killProcess(cmd)
}

func registerViaAvahi(path, hostname string, port int, version string) func() {
	cmd := exec.Command(path, hostname,
		"_codes._tcp",
		strconv.Itoa(port),
		fmt.Sprintf("port=%d", port),
		fmt.Sprintf("version=%s", version),
		fmt.Sprintf("host=%s.local", hostname),
	)
	if err := cmd.Start(); err != nil {
		log.Printf("[mDNS] Failed to start avahi-publish: %v", err)
		return func() {}
	}
	log.Printf("[mDNS] Registered '%s._codes._tcp.local' on port %d (avahi pid %d)", hostname, port, cmd.Process.Pid)
	return killProcess(cmd)
}

func killProcess(cmd *exec.Cmd) func() {
	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}
}

// parsePort extracts the numeric port from an address like ":8080" or "0.0.0.0:8080".
func parsePort(addr string) int {
	parts := strings.Split(addr, ":")
	if len(parts) == 0 {
		return 0
	}
	port, _ := strconv.Atoi(parts[len(parts)-1])
	return port
}
