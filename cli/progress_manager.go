package cli

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/chelnak/ysmrr"
)

// progressManagerKey is the context key for the ProgressManager.
type progressManagerKey struct{}

// ProgressManager wraps ysmrr.SpinnerManager with automatic cleanup, signal handling,
// and context integration. It manages the lifecycle of spinners and ensures proper
// terminal state restoration even when interrupted.
type ProgressManager struct {
	sm                  ysmrr.SpinnerManager
	started             bool
	stopped             bool
	mu                  sync.Mutex
	sigChan             chan os.Signal
	stopChan            chan struct{}
	cancellationMessage string
}

// NewProgressManager creates and initializes a ProgressManager with automatic signal handling.
// The manager will gracefully stop spinners on SIGINT/SIGTERM and restore terminal state.
func NewProgressManager() *ProgressManager {
	sm := &ProgressManager{
		sm:       ysmrr.NewSpinnerManager(),
		stopped:  false,
		sigChan:  make(chan os.Signal, 1),
		stopChan: make(chan struct{}),
	}

	// Set up signal handler for graceful shutdown on Ctrl+C
	signal.Notify(sm.sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sm.sigChan:
			sm.Stop()
			// Print cancellation message if set
			if sm.cancellationMessage != "" {
				errorf(os.Stderr, sm.cancellationMessage)
			}
			os.Exit(130) // Standard exit code for SIGINT (128 + 2)
		case <-func() <-chan struct{} {
			sm.mu.Lock()
			defer sm.mu.Unlock()
			if sm.stopChan != nil {
				return sm.stopChan
			}
			// If stopChan is nil, create a closed channel to unblock immediately
			closed := make(chan struct{})
			close(closed)
			return closed
		}():
			// Stop signal handler goroutine
			return
		}
	}()

	return sm
}

// Start begins the spinner rendering. Should be called after adding at least one spinner.
// If the manager was previously stopped, it will automatically create a fresh spinner manager
// to avoid re-rendering old spinners. Safe to call multiple times - will only start once.
func (s *ProgressManager) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we were stopped, create a fresh spinner manager to avoid re-rendering old spinners
	if s.stopped {
		s.sm = ysmrr.NewSpinnerManager()
		s.stopped = false
		s.started = false
	}

	// Only start if not already started (prevents re-rendering on multiple Start() calls)
	if !s.started {
		s.sm.Start()
		s.started = true
	}
}

// Stop stops all spinners and restores terminal state. Safe to call multiple times.
func (s *ProgressManager) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		s.stopped = true
		s.started = false
		s.sm.Stop()
	}
}

// StopSignalHandler stops the signal handler goroutine. This is useful for testing.
func (s *ProgressManager) StopSignalHandler() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopChan != nil {
		close(s.stopChan)
		s.stopChan = nil
	}
}

// IsStopped returns whether the manager has been stopped.
func (s *ProgressManager) IsStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

// AddSpinner creates and adds a new spinner with the given message.
func (s *ProgressManager) AddSpinner(message string) *ysmrr.Spinner {
	return s.sm.AddSpinner(message)
}

// SetCancellationMessage sets a custom message to display when the user interrupts (Ctrl+C).
// If not set, no message will be displayed on cancellation.
func (s *ProgressManager) SetCancellationMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancellationMessage = message
}

// WithProgressManager adds a ProgressManager to the context.
func WithProgressManager(ctx context.Context, pm *ProgressManager) context.Context {
	return context.WithValue(ctx, progressManagerKey{}, pm)
}

// GetProgressManager retrieves the ProgressManager from the context.
// Returns nil if no ProgressManager is present.
func GetProgressManager(ctx context.Context) *ProgressManager {
	if pm, ok := ctx.Value(progressManagerKey{}).(*ProgressManager); ok {
		return pm
	}
	return nil
}

// MustGetProgressManager retrieves the ProgressManager from the context.
// Panics if no ProgressManager is present. Use this when you're certain the
// context contains a ProgressManager.
func MustGetProgressManager(ctx context.Context) *ProgressManager {
	pm := GetProgressManager(ctx)
	if pm == nil {
		panic("ProgressManager not found in context")
	}
	return pm
}
