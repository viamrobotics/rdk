package cli

import (
	"fmt"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// StepStatus represents the state of a progress step.
type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepCompleted
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
	currentSpinner *pterm.SpinnerPrinter // Active child spinner (IndentLevel > 0)
	mu             sync.Mutex
}

// NewProgressManager creates a new ProgressManager with all steps registered upfront.
func NewProgressManager(steps []*Step) *ProgressManager {
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

	// If this is a parent step (IndentLevel == 0), print it as a static "in progress" indicator
	if step.IndentLevel == 0 {
		// Use ellipsis to indicate parent is in progress (with extra space for alignment)
		fmt.Printf(" …  %s\n", step.Message)
		return nil
	}

	// For child steps, stop the previous child spinner if one is active
	if pm.currentSpinner != nil {
		pm.currentSpinner.Stop()
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

	spinner, err := pterm.DefaultSpinner.
		WithRemoveWhenDone(false).
		WithText(adjustedPrefix + step.Message).
		Start()

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

	// If this is a parent step (IndentLevel == 0), update the static header
	if step.IndentLevel == 0 {
		// Count how many child lines were printed after the parent
		linesToMoveUp := 0
		foundParent := false
		for _, s := range pm.steps {
			if s.ID == stepID {
				foundParent = true
				continue
			}
			if foundParent && s.IndentLevel > 0 && (s.Status == StepCompleted || s.Status == StepRunning) {
				linesToMoveUp++
			}
		}

		// Move cursor up to the parent line, clear it, and print success
		if linesToMoveUp > 0 {
			fmt.Printf("\033[%dA", linesToMoveUp+1) // Move up
		}
		fmt.Printf("\r\033[K") // Clear line
		pterm.Success.Println(prefix + msg + elapsed)

		// Move cursor back down
		if linesToMoveUp > 0 {
			fmt.Printf("\033[%dB", linesToMoveUp) // Move down
		}
		return nil
	}

	// For child steps: if this is the currently active spinner, stop it and mark success
	if pm.currentSpinner != nil {
		pm.currentSpinner.Success(" " + prefix + msg + elapsed)
		pm.currentSpinner = nil
	} else {
		// If no spinner is active, just print the success message
		pterm.Success.Println(" " + prefix + msg + elapsed)
	}

	return nil
}

// CompleteWithMessage marks a step as completed with a custom message.
func (pm *ProgressManager) CompleteWithMessage(stepID string, message string) error {
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

	// If this is the currently active spinner, stop it and mark success
	if pm.currentSpinner != nil {
		pm.currentSpinner.Success(" " + prefix + message + elapsed)
		pm.currentSpinner = nil
	} else {
		// If no spinner is active, just print the success message
		pterm.Success.Println(" " + prefix + message + elapsed)
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

	step.Status = StepFailed

	msg := step.FailedMsg
	if msg == "" {
		msg = fmt.Sprintf("%s: %v", step.Message, err)
	}

	prefix := getPrefix(step)

	// If this is the currently active spinner, stop it and mark failure
	if pm.currentSpinner != nil {
		pm.currentSpinner.Fail(" " + prefix + msg)
		pm.currentSpinner = nil
	} else {
		// If no spinner is active, just print the error message
		pterm.Error.Println(" " + prefix + msg)
	}

	return nil
}

// FailWithMessage marks a step as failed with a custom message.
func (pm *ProgressManager) FailWithMessage(stepID string, message string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	step, exists := pm.stepMap[stepID]
	if !exists {
		return fmt.Errorf("step %q not found", stepID)
	}

	step.Status = StepFailed

	prefix := getPrefix(step)

	// If this is the currently active spinner, stop it and mark failure
	if pm.currentSpinner != nil {
		pm.currentSpinner.Fail(" " + prefix + message)
		pm.currentSpinner = nil
	} else {
		// If no spinner is active, just print the error message
		pterm.Error.Println(" " + prefix + message)
	}

	return nil
}

// UpdateText updates the text of the currently active spinner (for progress updates).
func (pm *ProgressManager) UpdateText(text string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.currentSpinner != nil {
		pm.currentSpinner.UpdateText(text)
	}
}

// Stop stops any active spinner.
func (pm *ProgressManager) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.currentSpinner != nil {
		pm.currentSpinner.Stop()
		pm.currentSpinner = nil
	}
}
