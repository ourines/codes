package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// JSONMode controls whether output is JSON or human-readable
var JSONMode bool

// Result represents a generic result for JSON output
type Result struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Print outputs data. In JSON mode, marshals to JSON. Otherwise calls the textFn.
func Print(data interface{}, textFn func()) {
	if JSONMode {
		out, err := json.MarshalIndent(Result{Success: true, Data: data}, "", "  ")
		if err != nil {
			PrintError(err)
			return
		}
		fmt.Println(string(out))
		return
	}
	textFn()
}

// PrintError outputs an error. In JSON mode, marshals error to JSON.
func PrintError(err error) {
	if JSONMode {
		out, _ := json.MarshalIndent(Result{Success: false, Error: err.Error()}, "", "  ")
		fmt.Println(string(out))
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
