package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ShowHeader(title string) {
	fmt.Printf(" %s\n", strings.Repeat("─", len(title)+2))
	fmt.Printf(" %s\n", title)
	fmt.Printf(" %s\n", strings.Repeat("─", len(title)+2))
}

func ShowConfigOption(num int, name, url string) {
	fmt.Printf("  %d. %s\n", num, name)
	fmt.Printf("     API: %s\n", url)
}

func ShowCurrentConfig(num int, name, url string) {
	fmt.Printf("  %d. %s (current)\n", num, name)
	fmt.Printf("     API: %s\n", url)
}

func ShowVersionItem(num int, version string) {
	fmt.Printf("  %d. %s\n", num, version)
}

func ShowLoading(format string, args ...interface{}) {
	fmt.Printf(" %s", fmt.Sprintf(format, args...))
	for i := 0; i < 3; i++ {
		fmt.Print(".")
	}
	fmt.Println()
}

func ShowSuccess(format string, args ...interface{}) {
	fmt.Printf(" ✓ %s\n", fmt.Sprintf(format, args...))
}

func ShowError(msg string, err error) {
	if err != nil {
		fmt.Printf(" ✗ %s: %v\n", msg, err)
	} else {
		fmt.Printf(" ✗ %s\n", msg)
	}
}

func ShowWarning(format string, args ...interface{}) {
	fmt.Printf(" ! %s\n", fmt.Sprintf(format, args...))
}

func ShowInfo(format string, args ...interface{}) {
	fmt.Printf(" ℹ %s\n", fmt.Sprintf(format, args...))
}

func CanWriteTo(dir string) bool {
	testFile := filepath.Join(dir, ".test_write")
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}
