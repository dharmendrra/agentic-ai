package tools

import "fmt"

// ErrToolNotFound returned when a tool doesn't exist
func ErrToolNotFound(name string) error {
	return fmt.Errorf("tool not found: %s", name)
}
