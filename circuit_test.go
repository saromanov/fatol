package fatol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCircuit(t *testing.T) {
	cb := NewCircuitBreaker()
	assert.Equal(t, cb.Stat().NumRequests, 0, "they should be equal")
}
