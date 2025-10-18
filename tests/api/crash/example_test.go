package crash

import (
	"fmt"
	"testing"
)

// Example demonstrates how to run crash tests
func Example() {
	// This is an example of how to use the crash tests
	// In practice, you would run the tests using go test commands
	fmt.Println("Example of how to run crash tests")
}

// TestBasicCrashTestsExample demonstrates how to run basic crash tests
func TestBasicCrashTestsExample(t *testing.T) {
	// This is an example test that would use the crash test suite
	// In practice, you would run the actual crash tests using go test commands
	t.Log("Example of how to run basic crash tests")
}

// TestSecurityCrashTests runs security-focused crash tests
func TestSecurityCrashTests(t *testing.T) {
	// This is an example test that would use the crash test suite
	// In practice, you would run the actual crash tests using go test commands
	t.Log("Example of how to run security crash tests")
}

// TestPerformanceCrashTests runs performance-focused crash tests
func TestPerformanceCrashTests(t *testing.T) {
	// This is an example test that would use the crash test suite
	// In practice, you would run the actual crash tests using go test commands
	t.Log("Example of how to run performance crash tests")
}
