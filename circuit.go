package fatol

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type contextFunc func(context.Context) error

var cb *CircuitBreaker

var errCompleteProcess = errors.New("unable to complete process")

// openStateError defines error
// in the case of openening state
type openStateError struct {
	requests uint32
}

// Error retruns message of the openStateError
func (e *openStateError) Error() string {
	return fmt.Sprintf("Circuit at the open state. Number of requests: %d", e.requests)
}

// requestsLimitError defines error
// in the case of half-openening state
type requestsLimitError struct {
	requests    uint32
	maxRequests uint32
}

// Error retruns message of the requestsLimitError
func (e *requestsLimitError) Error() string {
	return fmt.Sprintf("Requests limit is reached: %d: current requests: %d", e.maxRequests, e.requests)
}

// State represents type of the circuit state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// Strign returns current state of the ciruit
func (s State) String() string {
	switch s {
	case StateClosed:
		return "state is closed"
	case StateOpen:
		return "state is open"
	case StateHalfOpen:
		return "state is half-open"
	default:
		return fmt.Sprintf("unknown state: %d", s)
	}
}

// CircuitBreaker is a struct for implementing of circuit breaker
type CircuitBreaker struct {
	Timeout               time.Time
	mutex                 *sync.Mutex
	timeout               time.Time
	openInterval          time.Duration
	expiry                time.Time
	lastStateChanged      time.Time
	maxRequests           uint32
	numFailedRequests     uint32
	numSuccessfulRequests uint32
	numRequests           uint32
	state                 State
}

// NewCircuitBreaker returns new curcuit breaker object
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		mutex:        &sync.Mutex{},
		openInterval: 1 * time.Minute,
		maxRequests:  5,
		state:        StateClosed,
	}
}
func (cb *CircuitBreaker) newObject() {
	cb.clear()
	switch cb.state {
	case StateOpen:
		cb.expiry.Add(cb.openInterval)
	case StateHalfOpen:
		cb.expiry = time.Time{}
	}
}

func (cb *CircuitBreaker) setState(state State) {

	if cb.state == state {
		return
	}
	switch state {
	case StateOpen:
		cb.timeout = time.Now()
		cb.lastStateChanged = time.Now().UTC()
	case StateClosed:
		cb.timeout = time.Now()
		cb.lastStateChanged = time.Now().UTC()
	}
	cb.state = state
}
func (cb *CircuitBreaker) clear() {
	cb.numFailedRequests = 0
	cb.numSuccessfulRequests = 0
	cb.numRequests = 0
}

func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.numRequests++
	switch cb.state {
	case StateOpen:
		return 0, &openStateError{
			requests: cb.numRequests,
		}
	case StateHalfOpen:
		if cb.numRequests > cb.maxRequests {
			return 0, &requestsLimitError{
				requests:    cb.numRequests,
				maxRequests: cb.maxRequests,
			}
		}
	}
	return 0, nil
}

// In the case if state was closed and operation was failed
// then make state as open
// In the case if state was half-open and operation was failed
// then make state as open too
func (cb *CircuitBreaker) onFailure() {
	cb.numRequests++
	cb.numFailedRequests++
	switch cb.state {
	case StateClosed:
		cb.setState(StateOpen)
	case StateHalfOpen:
		cb.setState(StateOpen)
	}
}

func (cb *CircuitBreaker) onSuccessful() {
	cb.numRequests++
	cb.numSuccessfulRequests++
}

func (cb *CircuitBreaker) currentState() State {
	now := time.Now()
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.newObject()
		}
	case StateOpen:
		if cb.expiry.After(now) {
			cb.setState(StateHalfOpen)
			cb.lastStateChanged = time.Now().UTC()
		}
	}
	return cb.state
}

func (cb *CircuitBreaker) recordRequest(err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	if err == nil {
		cb.onSuccessful()
		return
	}

	cb.onFailure()
}

// Do implements main method for circuit breaker
func (cb *CircuitBreaker) Do(req func() (interface{}, error)) (interface{}, error) {
	_, err := cb.beforeRequest()
	switch err := err.(type) {
	case *openStateError:
		return nil, errors.Wrap(err, "unable to process request at the open state")
	case *requestsLimitError:
		return nil, errors.Wrap(err, "unable to process request with reaching limit")
	default:

	}

	defer func() {
		err := recover()
		if err != nil {
			cb.recordRequest(errCompleteProcess)
			panic(err)
		}
	}()

	result, err := req()
	cb.recordRequest(err)
	return result, err
}

// readyToTrip provides deafault ready to trip
// implementation
func readyToTrip(cb CircuitBreaker) bool {
	return cb.numFailedRequests > 3
}
