package balancer

import "fmt"

// NewFromName returns the Algorithm implementation for a config name.
func NewFromName(name string) (Algorithm, error) {
	switch name {
	case "round_robin":
		return &RoundRobin{}, nil
	default:
		return nil, fmt.Errorf("balancer: unknown algorithm %q", name)
	}
}
