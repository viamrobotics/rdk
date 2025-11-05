package cli

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"
)

type fakeSpinner struct {
	mu        sync.Mutex
	text      string
	stopped   bool
	successes []string
	failures  []string
}

func (f *fakeSpinner) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = true
	return nil
}

func (f *fakeSpinner) Success(message ...any) {
	f.mu.Lock()
	if len(message) > 0 {
		f.successes = append(f.successes, fmt.Sprint(message...))
	} else {
		f.successes = append(f.successes, "")
	}
	f.mu.Unlock()
	_ = f.Stop()
}

func (f *fakeSpinner) Fail(message ...any) {
	f.mu.Lock()
	if len(message) > 0 {
		f.failures = append(f.failures, fmt.Sprint(message...))
	} else {
		f.failures = append(f.failures, "")
	}
	f.mu.Unlock()
	_ = f.Stop()
}

func (f *fakeSpinner) UpdateText(text string) {
	f.mu.Lock()
	f.text = text
	f.mu.Unlock()
}

func newFakeSpinnerFactory() progressSpinnerFactory {
	return func(text string) (progressSpinner, error) {
		fs := &fakeSpinner{}
		fs.UpdateText(text)
		return fs, nil
	}
}

func newTestProgressManager(steps []*Step, opts ...ProgressManagerOption) *ProgressManager {
	opts = append(opts, withProgressSpinnerFactory(newFakeSpinnerFactory()))
	return NewProgressManager(steps, opts...)
}

func TestNewProgressManager(t *testing.T) {
	steps := []*Step{
		{ID: "parent1", Message: "Parent step", IndentLevel: 0},
		{ID: "child1", Message: "Child step", IndentLevel: 1},
		{ID: "child2", Message: "Another child", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)
	defer pm.Stop() // Clean up any active spinners

	if pm == nil {
		t.Fatal("NewProgressManager returned nil")
	}

	if len(pm.steps) != 3 {
		t.Errorf("Expected 3 steps, got %d", len(pm.steps))
	}

	if len(pm.stepMap) != 3 {
		t.Errorf("Expected 3 step map entries, got %d", len(pm.stepMap))
	}

	if pm.currentSpinner != nil {
		t.Error("Expected currentSpinner to be nil initially")
	}

	// Verify step map contains all steps
	for _, step := range steps {
		if _, exists := pm.stepMap[step.ID]; !exists {
			t.Errorf("Step %q not found in step map", step.ID)
		}
	}
}

func TestGetPrefix(t *testing.T) {
	tests := []struct {
		step     *Step
		expected string
	}{
		{&Step{ID: "root", IndentLevel: 0}, ""},
		{&Step{ID: "child1", IndentLevel: 1}, "  → "},
		{&Step{ID: "child2", IndentLevel: 2}, "    → "},
		{&Step{ID: "child3", IndentLevel: 3}, "      → "},
	}

	for _, test := range tests {
		result := getPrefix(test.step)
		if result != test.expected {
			t.Errorf("getPrefix(%v) = %q, expected %q", test.step.IndentLevel, result, test.expected)
		}
	}
}

func TestStartParentStep(t *testing.T) {
	steps := []*Step{
		{ID: "parent", Message: "Parent step", IndentLevel: 0},
	}

	pm := newTestProgressManager(steps)

	err := pm.Start("parent")
	if err != nil {
		t.Fatalf("Failed to start parent step: %v", err)
	}

	step := pm.stepMap["parent"]
	if step.Status != StepRunning {
		t.Errorf("Expected step status to be StepRunning, got %v", step.Status)
	}

	if step.startTime.IsZero() {
		t.Error("Expected startTime to be set for parent step")
	}
}

func TestStartChildStep(t *testing.T) {
	steps := []*Step{
		{ID: "child", Message: "Child step", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)
	defer pm.Stop() // Clean up any active spinners

	err := pm.Start("child")
	if err != nil {
		t.Fatalf("Failed to start child step: %v", err)
	}

	if pm.currentSpinner == nil {
		t.Error("Expected currentSpinner to be set for child step")
	}

	step := pm.stepMap["child"]
	if step.Status != StepRunning {
		t.Errorf("Expected step status to be StepRunning, got %v", step.Status)
	}
}

func TestProgressManagerWithOutputDisabled(t *testing.T) {
	steps := []*Step{
		{ID: "parent", Message: "Parent", IndentLevel: 0},
		{ID: "child", Message: "Child", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps, WithProgressOutput(false))
	defer pm.Stop()

	err := pm.Start("parent")
	if err != nil {
		t.Fatalf("Failed to start parent step: %v", err)
	}

	if pm.currentSpinner != nil {
		t.Fatal("Expected no spinner when output is disabled for parent step")
	}

	err = pm.Start("child")
	if err != nil {
		t.Fatalf("Failed to start child step: %v", err)
	}

	if pm.currentSpinner != nil {
		t.Fatal("Expected no spinner when output is disabled for child step")
	}

	err = pm.Complete("child")
	if err != nil {
		t.Fatalf("Failed to complete child step: %v", err)
	}

	err = pm.Complete("parent")
	if err != nil {
		t.Fatalf("Failed to complete parent step: %v", err)
	}

	test.That(t, pm.stepMap["parent"].Status, test.ShouldEqual, StepCompleted)
	test.That(t, pm.stepMap["child"].Status, test.ShouldEqual, StepCompleted)
}

func TestStartInvalidStep(t *testing.T) {
	steps := []*Step{
		{ID: "valid", Message: "Valid step", IndentLevel: 0},
	}

	pm := newTestProgressManager(steps)

	err := pm.Start("invalid")
	if err == nil {
		t.Error("Expected error for invalid step ID")
	}

	expectedError := "step \"invalid\" not found"
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got %q", expectedError, err.Error())
	}
}

func TestStartReplacesPreviousSpinner(t *testing.T) {
	steps := []*Step{
		{ID: "child1", Message: "First child", IndentLevel: 1},
		{ID: "child2", Message: "Second child", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)
	defer pm.Stop() // Clean up any active spinners

	// Start first child
	err := pm.Start("child1")
	if err != nil {
		t.Fatalf("Failed to start first child: %v", err)
	}

	firstSpinner := pm.currentSpinner

	// Start second child
	err = pm.Start("child2")
	if err != nil {
		t.Fatalf("Failed to start second child: %v", err)
	}

	// First spinner should be stopped and replaced
	if pm.currentSpinner == firstSpinner {
		t.Error("Expected currentSpinner to be replaced with new spinner")
	}
}

func TestCompleteParentStep(t *testing.T) {
	steps := []*Step{
		{ID: "parent", Message: "Parent step", CompletedMsg: "Parent completed", IndentLevel: 0},
		{ID: "child1", Message: "Child 1", IndentLevel: 1},
		{ID: "child2", Message: "Child 2", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)

	// Complete some child steps first
	pm.stepMap["child1"].Status = StepCompleted
	pm.stepMap["child2"].Status = StepRunning

	err := pm.Start("parent")
	if err != nil {
		t.Fatalf("Failed to start parent step: %v", err)
	}

	err = pm.Complete("parent")
	if err != nil {
		t.Fatalf("Failed to complete parent step: %v", err)
	}

	step := pm.stepMap["parent"]
	if step.Status != StepCompleted {
		t.Errorf("Expected step status to be StepCompleted, got %v", step.Status)
	}

	if step.startTime.IsZero() {
		t.Error("Expected startTime to be set for parent step")
	}
}

func TestCompleteChildStep(t *testing.T) {
	steps := []*Step{
		{ID: "child", Message: "Child step", CompletedMsg: "Child completed", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)
	defer pm.Stop() // Clean up any active spinners

	err := pm.Start("child")
	if err != nil {
		t.Fatalf("Failed to start child step: %v", err)
	}

	err = pm.Complete("child")
	if err != nil {
		t.Fatalf("Failed to complete child step: %v", err)
	}

	step := pm.stepMap["child"]
	if step.Status != StepCompleted {
		t.Errorf("Expected step status to be StepCompleted, got %v", step.Status)
	}

	if pm.currentSpinner != nil {
		t.Error("Expected currentSpinner to be nil after completion")
	}
}

func TestCompleteWithElapsedTime(t *testing.T) {
	steps := []*Step{
		{ID: "child", Message: "Child step", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)
	defer pm.Stop() // Clean up any active spinners

	err := pm.Start("child")
	if err != nil {
		t.Fatalf("Failed to start child step: %v", err)
	}

	// Wait a bit to ensure measurable elapsed time
	time.Sleep(10 * time.Millisecond)

	err = pm.Complete("child")
	if err != nil {
		t.Fatalf("Failed to complete child step: %v", err)
	}

	// Check that elapsed time was calculated and included
	// This is a bit tricky to test directly since we can't capture pterm output easily
	// But we can verify the step timing was recorded
	step := pm.stepMap["child"]
	if step.startTime.IsZero() {
		t.Error("Expected startTime to be set")
	}

	elapsed := time.Since(step.startTime)
	test.That(t, elapsed, test.ShouldBeGreaterThanOrEqualTo, 10*time.Millisecond)
}

func TestCompleteWithMessage(t *testing.T) {
	steps := []*Step{
		{ID: "child", Message: "Child step", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)

	err := pm.Start("child")
	if err != nil {
		t.Fatalf("Failed to start child step: %v", err)
	}

	customMessage := "Custom completion message"
	err = pm.CompleteWithMessage("child", customMessage)
	if err != nil {
		t.Fatalf("Failed to complete child step with message: %v", err)
	}

	if pm.currentSpinner != nil {
		t.Error("Expected currentSpinner to be nil after completion")
	}
}

func TestFailParentStep(t *testing.T) {
	steps := []*Step{
		{ID: "parent", Message: "Parent step", FailedMsg: "Parent failed", IndentLevel: 0},
	}

	pm := newTestProgressManager(steps)

	err := pm.Start("parent")
	if err != nil {
		t.Fatalf("Failed to start parent step: %v", err)
	}

	testErr := fmt.Errorf("test error")
	err = pm.Fail("parent", testErr)
	if err != nil {
		t.Fatalf("Failed to fail parent step: %v", err)
	}

	step := pm.stepMap["parent"]
	if step.Status != StepFailed {
		t.Errorf("Expected step status to be StepFailed, got %v", step.Status)
	}
}

func TestFailChildStep(t *testing.T) {
	steps := []*Step{
		{ID: "child", Message: "Child step", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)

	err := pm.Start("child")
	if err != nil {
		t.Fatalf("Failed to start child step: %v", err)
	}

	testErr := fmt.Errorf("test error")
	err = pm.Fail("child", testErr)
	if err != nil {
		t.Fatalf("Failed to fail child step: %v", err)
	}

	step := pm.stepMap["child"]
	if step.Status != StepFailed {
		t.Errorf("Expected step status to be StepFailed, got %v", step.Status)
	}

	if pm.currentSpinner != nil {
		t.Error("Expected currentSpinner to be nil after failure")
	}
}

func TestFailWithMessage(t *testing.T) {
	steps := []*Step{
		{ID: "child", Message: "Child step", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)

	err := pm.Start("child")
	if err != nil {
		t.Fatalf("Failed to start child step: %v", err)
	}

	customMessage := "Custom failure message"
	err = pm.FailWithMessage("child", customMessage)
	if err != nil {
		t.Fatalf("Failed to fail child step with message: %v", err)
	}

	if pm.currentSpinner != nil {
		t.Error("Expected currentSpinner to be nil after failure")
	}
}

func TestFailWithoutCustomMessage(t *testing.T) {
	steps := []*Step{
		{ID: "child", Message: "Child step", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)

	err := pm.Start("child")
	if err != nil {
		t.Fatalf("Failed to start child step: %v", err)
	}

	testErr := fmt.Errorf("test error")
	err = pm.Fail("child", testErr)
	if err != nil {
		t.Fatalf("Failed to fail child step: %v", err)
	}

	// Verify that the error message includes both the step message and the error
	step := pm.stepMap["child"]
	expectedMsg := fmt.Sprintf("%s: %v", step.Message, testErr)
	if step.FailedMsg != "" {
		// If FailedMsg is set, it should be used instead
		if step.FailedMsg != expectedMsg {
			t.Errorf("Expected failed message to be %q, got %q", expectedMsg, step.FailedMsg)
		}
	}
}

func TestStop(t *testing.T) {
	steps := []*Step{
		{ID: "child", Message: "Child step", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)
	defer pm.Stop() // Clean up any active spinners

	err := pm.Start("child")
	if err != nil {
		t.Fatalf("Failed to start child step: %v", err)
	}

	if pm.currentSpinner == nil {
		t.Fatal("Expected currentSpinner to be set")
	}

	pm.Stop()

	if pm.currentSpinner != nil {
		t.Error("Expected currentSpinner to be nil after stop")
	}
}

func TestConcurrentAccess(t *testing.T) {
	steps := []*Step{
		{ID: "child1", Message: "Child 1", IndentLevel: 1},
		{ID: "child2", Message: "Child 2", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)
	defer pm.Stop() // Clean up any active spinners

	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent Start calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			// Alternate between two child steps
			stepID := "child1"
			if index%2 == 0 {
				stepID = "child2"
			}
			pm.Start(stepID)
		}(i)
	}

	wg.Wait()

	// Test concurrent Complete calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			// Alternate between two child steps
			stepID := "child1"
			if index%2 == 0 {
				stepID = "child2"
			}
			pm.Complete(stepID)
		}(i)
	}

	wg.Wait()

	// Verify no data races occurred and final state is consistent
	if pm.currentSpinner != nil {
		t.Error("Expected currentSpinner to be nil after all operations")
	}
}

func TestStepStatusTransitions(t *testing.T) {
	steps := []*Step{
		{ID: "step1", Message: "Step 1", IndentLevel: 1},
		{ID: "step2", Message: "Step 2", IndentLevel: 0},
	}

	pm := newTestProgressManager(steps)
	defer pm.Stop() // Clean up any active spinners

	// Test initial status
	for _, step := range steps {
		if step.Status != StepPending {
			t.Errorf("Expected initial status to be StepPending, got %v", step.Status)
		}
	}

	// Test transition to running
	err := pm.Start("step1")
	if err != nil {
		t.Fatalf("Failed to start step1: %v", err)
	}

	if pm.stepMap["step1"].Status != StepRunning {
		t.Errorf("Expected step1 status to be StepRunning, got %v", pm.stepMap["step1"].Status)
	}

	// Test transition to completed
	err = pm.Complete("step1")
	if err != nil {
		t.Fatalf("Failed to complete step1: %v", err)
	}

	if pm.stepMap["step1"].Status != StepCompleted {
		t.Errorf("Expected step1 status to be StepCompleted, got %v", pm.stepMap["step1"].Status)
	}

	// Test transition to failed
	err = pm.Start("step2")
	if err != nil {
		t.Fatalf("Failed to start step2: %v", err)
	}

	testErr := fmt.Errorf("test error")
	err = pm.Fail("step2", testErr)
	if err != nil {
		t.Fatalf("Failed to fail step2: %v", err)
	}

	if pm.stepMap["step2"].Status != StepFailed {
		t.Errorf("Expected step2 status to be StepFailed, got %v", pm.stepMap["step2"].Status)
	}
}

func TestEmptySteps(t *testing.T) {
	pm := newTestProgressManager([]*Step{})

	if len(pm.steps) != 0 {
		t.Errorf("Expected 0 steps, got %d", len(pm.steps))
	}

	if len(pm.stepMap) != 0 {
		t.Errorf("Expected 0 step map entries, got %d", len(pm.stepMap))
	}
}

func TestMultipleOperationsOnSameStep(t *testing.T) {
	steps := []*Step{
		{ID: "step", Message: "Test step", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)
	defer pm.Stop() // Clean up any active spinners

	// Start the step
	err := pm.Start("step")
	if err != nil {
		t.Fatalf("Failed to start step: %v", err)
	}

	// Complete it
	err = pm.Complete("step")
	if err != nil {
		t.Fatalf("Failed to complete step: %v", err)
	}

	// Try to complete it again (should still work)
	err = pm.Complete("step")
	if err != nil {
		t.Errorf("Expected second complete to succeed, got error: %v", err)
	}

	// Start it again
	err = pm.Start("step")
	if err != nil {
		t.Fatalf("Failed to restart step: %v", err)
	}

	// Fail it
	testErr := fmt.Errorf("test error")
	err = pm.Fail("step", testErr)
	if err != nil {
		t.Fatalf("Failed to fail step: %v", err)
	}
}

func TestStopAndRestartSpinner(t *testing.T) {
	steps := []*Step{
		{ID: "step", Message: "Test step", IndentLevel: 1},
	}

	pm := newTestProgressManager(steps)
	defer pm.Stop() // Clean up any active spinners

	// Start the step
	err := pm.Start("step")
	if err != nil {
		t.Fatalf("Failed to start step: %v", err)
	}

	// Verify spinner is active
	if pm.currentSpinner == nil {
		t.Fatal("Expected currentSpinner to be set")
	}

	// Stop the spinner
	pm.Stop()

	// Verify spinner is cleaned up
	if pm.currentSpinner != nil {
		t.Error("Expected currentSpinner to be nil after stop")
	}

	// Start it again
	err = pm.Start("step")
	if err != nil {
		t.Fatalf("Failed to restart step: %v", err)
	}

	// Verify spinner is active again
	if pm.currentSpinner == nil {
		t.Error("Expected currentSpinner to be set after restart")
	}

	// Complete it
	err = pm.Complete("step")
	if err != nil {
		t.Fatalf("Failed to complete step: %v", err)
	}

	// Verify spinner is cleaned up after completion
	if pm.currentSpinner != nil {
		t.Error("Expected currentSpinner to be nil after completion")
	}
}
