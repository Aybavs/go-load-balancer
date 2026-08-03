package balancer

import "fmt"

// NewFromName returns the Algorithm implementation for a config name.
func NewFromName(name string) (Algorithm, error) {
	switch name {
	case "round_robin":
		return &RoundRobin{}, nil
	case "least_connections":
		return LeastConnections{}, nil
	case "consistent_hash":
		return NewConsistentHash(), nil
	default:
		return nil, fmt.Errorf("balancer: unknown algorithm %q", name)
	}
}
