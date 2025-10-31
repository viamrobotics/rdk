package cli

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

type progressSpinner interface {
	Stop() error
	Success(...any)
	Fail(...any)
	UpdateText(string)
}

type progressSpinnerFactory func(string) (progressSpinner, error)

var defaultSpinnerFactory progressSpinnerFactory = func(text string) (progressSpinner, error) {
	spinner, err := pterm.DefaultSpinner.
		WithRemoveWhenDone(false).
		WithText(text).
		Start()
	if err != nil {
		return nil, err
	}
	return spinner, nil
}

// StepStatus represents the state of a progress step.
type StepStatus int

const (
	// StepPending indicates a step has not yet started.
	StepPending StepStatus = iota
	// StepRunning indicates a step is currently in progress.
	StepRunning
	// StepCompleted indicates a step finished successfully.
	StepCompleted
	// StepFailed indicates a step encountered an error.
	StepFailed
)

// Step represents a single progress step.
type Step struct {
	ID           string
	Message      string
	Status       StepStatus
	CompletedMsg string    // Optional: Custom message when completed
	FailedMsg    string    // Optional: Custom message when failed
	IndentLevel  int       // 0 = root, 1 = child (→), 2 = nested child, etc.
	startTime    time.Time // Internal: when the step started
}

// ProgressManager manages a sequence of steps with spinners (sequential display).
type ProgressManager struct {
	steps          []*Step
	stepMap        map[string]*Step
	currentSpinner progressSpinner // Active child spinner (IndentLevel > 0)
	spinnerFactory progressSpinnerFactory
	mu             sync.Mutex
	disabled       bool
}

// ProgressManagerOption allows customizing ProgressManager behavior at creation time.
type ProgressManagerOption func(*ProgressManager)

// WithProgressOutput enables or disables terminal output for a ProgressManager.
func WithProgressOutput(enabled bool) ProgressManagerOption {
	return func(pm *ProgressManager) {
		pm.disabled = !enabled
	}
}

func withProgressSpinnerFactory(factory progressSpinnerFactory) ProgressManagerOption {
	return func(pm *ProgressManager) {
		pm.spinnerFactory = factory
	}
}

// NewProgressManager creates a new ProgressManager with all steps registered upfront.
func NewProgressManager(steps []*Step, opts ...ProgressManagerOption) *ProgressManager {
	// Customize spinner style globally
	pterm.Success.Prefix = pterm.Prefix{
		Text:  "✓",
		Style: pterm.NewStyle(pterm.FgGreen),
	}
	pterm.Error.Prefix = pterm.Prefix{
		Text:  "✗",
		Style: pterm.NewStyle(pterm.FgRed),
	}
	// Add a leading space to each spinner sequence character for alignment
	baseSequence := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerSequence := make([]string, len(baseSequence))
	for i, char := range baseSequence {
		spinnerSequence[i] = " " + char
	}
	pterm.DefaultSpinner.Sequence = spinnerSequence
	pterm.DefaultSpinner.Style = pterm.NewStyle(pterm.FgCyan)

	stepMap := make(map[string]*Step)
	for _, step := range steps {
		stepMap[step.ID] = step
	}

	pm := &ProgressManager{
		steps:          steps,
		stepMap:        stepMap,
		currentSpinner: nil,
		spinnerFactory: defaultSpinnerFactory,
	}

	for _, opt := range opts {
		opt(pm)
	}

	return pm
}

// getPrefix returns the formatted prefix for a step based on its indent level.
func getPrefix(step *Step) string {
	prefix := ""
	for i := 0; i < step.IndentLevel; i++ {
		prefix += "  "
	}
	if step.IndentLevel > 0 {
		prefix += "→ "
	}
	return prefix
}

// Start begins animating the spinner for the given step ID.
func (pm *ProgressManager) Start(stepID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	step, exists := pm.stepMap[stepID]
	if !exists {
		return fmt.Errorf("step %q not found", stepID)
	}

	step.Status = StepRunning
	step.startTime = time.Now() // Record start time

	if pm.disabled {
		return nil
	}

	// If this is a parent step (IndentLevel == 0), print "…" indicator
	if step.IndentLevel == 0 {
		_, _ = os.Stdout.WriteString(fmt.Sprintf(" …  %s\n", step.Message)) //nolint:errcheck
		return nil
	}

	// For child steps, stop the previous child spinner if one is active
	if pm.currentSpinner != nil {
		_ = pm.currentSpinner.Stop() //nolint:errcheck
	}

	// Create and start a new spinner for this child step
	// pterm adds an automatic space after the spinner character, so we need to
	// add one MORE space to the prefix to match the completed format
	adjustedPrefix := ""
	for i := 0; i < step.IndentLevel; i++ {
		adjustedPrefix += "  "
	}
	if step.IndentLevel > 0 {
		adjustedPrefix += "  → " // Three spaces before arrow to match completed format
	}

	if pm.spinnerFactory == nil {
		pm.spinnerFactory = defaultSpinnerFactory
	}

	spinner, err := pm.spinnerFactory(adjustedPrefix + step.Message)
	if err != nil {
		return fmt.Errorf("failed to start child spinner: %w", err)
	}

	pm.currentSpinner = spinner

	return nil
}

// Complete marks a step as completed with a success message.
func (pm *ProgressManager) Complete(stepID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	step, exists := pm.stepMap[stepID]
	if !exists {
		return fmt.Errorf("step %q not found", stepID)
	}

	step.Status = StepCompleted

	msg := step.CompletedMsg
	if msg == "" {
		msg = step.Message
	}

	// Add elapsed time if step was started
	elapsed := ""
	if !step.startTime.IsZero() {
		duration := time.Since(step.startTime)
		elapsed = fmt.Sprintf(" (%s)", duration.Round(time.Second))
	}

	prefix := getPrefix(step)

	if pm.disabled {
		return nil
	}

	// For both parent and child steps: if this is the currently active spinner, stop it and mark success
	if pm.currentSpinner != nil {
		if step.IndentLevel == 0 {
			// Parent steps don't need extra leading space since pterm adds it
			pm.currentSpinner.Success(msg + elapsed)
		} else {
			pm.currentSpinner.Success(" " + prefix + msg + elapsed)
		}
		pm.currentSpinner = nil
	} else {
		// If no spinner is active, just print the success message
		if step.IndentLevel == 0 {
			// Parent steps don't need extra leading space since pterm adds it
			pterm.Success.Println(msg + elapsed)
		} else {
			// Child steps need extra leading space and prefix
			pterm.Success.Println(" " + prefix + msg + elapsed)
		}
	}

	return nil
}

// CompleteWithMessage marks a step as completed with a custom message.
func (pm *ProgressManager) CompleteWithMessage(stepID, message string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	step, exists := pm.stepMap[stepID]
	if !exists {
		return fmt.Errorf("step %q not found", stepID)
	}

	step.Status = StepCompleted

	// Add elapsed time if step was started
	elapsed := ""
	if !step.startTime.IsZero() {
		duration := time.Since(step.startTime)
		elapsed = fmt.Sprintf(" (%s)", duration.Round(time.Second))
	}

	prefix := getPrefix(step)

	if pm.disabled {
		return nil
	}

	// If this is the currently active spinner, stop it and mark success
	if pm.currentSpinner != nil {
		pm.currentSpinner.Success(" " + prefix + message + elapsed)
		pm.currentSpinner = nil
	} else {
		// If no spinner is active, just print the success message
		// Parent steps (IndentLevel 0) don't need extra leading space since pterm adds it
		if step.IndentLevel == 0 {
			pterm.Success.Println(message + elapsed)
		} else {
			pterm.Success.Println(" " + prefix + message + elapsed)
		}
	}

	return nil
}

// Fail marks a step as failed with an error message.
func (pm *ProgressManager) Fail(stepID string, err error) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	step, exists := pm.stepMap[stepID]
	if !exists {
		return fmt.Errorf("step %q not found", stepID)
	}

	msg := step.FailedMsg
	if msg == "" {
		msg = fmt.Sprintf("%s: %v", step.Message, err)
	}

	pm.failWithMessageLocked(step, msg)
	return nil
}

// FailWithMessage marks a step as failed with a custom message.
func (pm *ProgressManager) FailWithMessage(stepID, message string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	step, exists := pm.stepMap[stepID]
	if !exists {
		return fmt.Errorf("step %q not found", stepID)
	}

	pm.failWithMessageLocked(step, message)
	return nil
}

// failWithMessageLocked is the shared implementation for failing a step.
// It assumes the lock is already held by the caller.
func (pm *ProgressManager) failWithMessageLocked(step *Step, message string) {
	step.Status = StepFailed

	if pm.disabled {
		return
	}

	prefix := getPrefix(step)

	// If this is the currently active spinner, stop it and mark failure
	if pm.currentSpinner != nil {
		pm.currentSpinner.Fail(" " + prefix + message)
		pm.currentSpinner = nil
	} else {
		// If no spinner is active, just print the error message
		// Parent steps (IndentLevel 0) don't need extra leading space since pterm adds it
		if step.IndentLevel == 0 {
			pterm.Error.Println(message)
		} else {
			pterm.Error.Println(" " + prefix + message)
		}
	}
}

// UpdateText updates the text of the currently active spinner (for progress updates).
func (pm *ProgressManager) UpdateText(text string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.disabled {
		return
	}

	if pm.currentSpinner != nil {
		pm.currentSpinner.UpdateText(text)
	}
}

// Stop stops any active spinner.
func (pm *ProgressManager) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.disabled {
		return
	}

	if pm.currentSpinner != nil {
		_ = pm.currentSpinner.Stop() //nolint:errcheck
		pm.currentSpinner = nil
	}
}
