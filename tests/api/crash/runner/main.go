package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func main() {
	var (
		baseURL = flag.String("url", "http://localhost:8080", "Base URL of the FSM API server")
		timeout = flag.Duration("timeout", 60*time.Second, "Test timeout duration")
		verbose = flag.Bool("verbose", false, "Enable verbose output")
		pattern = flag.String("pattern", "", "Run only tests matching the pattern")
	)
	flag.Parse()

	fmt.Println("FSM API Crash Test Runner")
	fmt.Println("========================")
	fmt.Printf("Target URL: %s\n", *baseURL)
	fmt.Printf("Timeout: %v\n", *timeout)
	if *pattern != "" {
		fmt.Printf("Pattern: %s\n", *pattern)
	}
	fmt.Println("")

	// Check if the API server is running
	fmt.Println("Checking if API server is running...")
	cmd := exec.Command("curl", "-s", "-f", *baseURL+"/api/v1/health")
	if err := cmd.Run(); err != nil {
		fmt.Printf("ERROR: API server is not running at %s\n", *baseURL)
		fmt.Println("Please start the FSM API server first:")
		fmt.Println("  go run cmd/api/main.go")
		os.Exit(1)
	}
	fmt.Println("✓ API server is running")

	// Build test command
	args := []string{"test", "-timeout", timeout.String()}
	if *verbose {
		args = append(args, "-v")
	}
	if *pattern != "" {
		args = append(args, "-run", *pattern)
	}
	args = append(args, "../crash")

	// Run the tests
	fmt.Println("")
	fmt.Println("Running crash tests...")
	fmt.Println("=====================")

	cmd = exec.Command("go", args...)
	cmd.Dir = "/Users/rstyczynski/projects/ansible_fsm/fsm_v2/tests/api/crash/runner"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("Tests failed: %v", err)
	}

	fmt.Println("")
	fmt.Println("Crash tests completed!")
}
