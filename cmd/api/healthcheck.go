package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// probe performs a local readiness request and maps it to an exit code.
//
// A scratch image has no shell, curl or wget, so the binary acts as its own
// container HEALTHCHECK via `app -healthcheck`.
func probe() int {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health/ready", port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.StatusCode)
		return 1
	}
	return 0
}
