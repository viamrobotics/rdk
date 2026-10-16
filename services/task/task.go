// Package task defines the task service: a unit of robot work with a clear
// start, a clear end and an outcome. A task runs once per Run call, reports
// progress while it runs, and returns an outcome plus output data. Tasks emit
// outcomes; a state machine owns what happens next.
package task

import (
	"context"
	"errors"
	"time"

	pb "go.viam.com/api/service/task/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterTaskServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.TaskService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is the name of the type of service.
const SubtypeName = "task"

// API is a variable that identifies the task service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Outcomes every task author and state machine can rely on. A task may return
// any other outcome name; these are the ones with shared meaning.
const (
	// OutcomeDone is the outcome of a task that finished what it set out to do.
	OutcomeDone = "done"
	// OutcomeError is the outcome a state machine assigns to a task that
	// returned a Go error (a gRPC error over the wire).
	OutcomeError = "error"
	// OutcomeTimeout is the outcome a state machine assigns when a state's
	// timeout cancels its task.
	OutcomeTimeout = "timeout"
	// OutcomeSatisfied is the outcome of a wait_for task whose condition held.
	OutcomeSatisfied = "satisfied"
)

// ErrAlreadyRunning is returned by Run when an execution is already in flight.
// Only one execution may run at a time per task resource.
var ErrAlreadyRunning = errors.New("task is already running")

// Phase is whether a task currently has an execution in flight.
type Phase int

const (
	// PhaseIdle means no execution is in flight.
	PhaseIdle Phase = iota
	// PhaseRunning means an execution is in flight.
	PhaseRunning
)

// String returns the lower case name of the phase.
func (p Phase) String() string {
	if p == PhaseRunning {
		return "running"
	}
	return "idle"
}

// Progress is a non-terminal update from a running task.
type Progress struct {
	// Message is a human readable description of what the task is doing.
	Message string
	// Fraction is how complete the task is, in [0, 1]. 0 when unknown.
	Fraction float64
}

// Result is the terminal outcome of one execution.
type Result struct {
	// Outcome is the outcome name a state machine looks up in its transitions.
	Outcome string
	// Output is data a state machine merges into its context.
	Output map[string]interface{}
}

// Done returns a Result with OutcomeDone and the given output.
func Done(output map[string]interface{}) Result {
	return Result{Outcome: OutcomeDone, Output: output}
}

// Status is a read-only view of a task for UIs and agents.
type Status struct {
	Phase Phase
	// LastResult is the result of the most recent completed execution, if any.
	LastResult *Result
	// StartedAt is when the current execution started. Zero when idle.
	StartedAt time.Time
	// Progress is the current execution's latest progress. Nil when idle or
	// when the task has not reported any.
	Progress *Progress
}

// ProgressFunc receives progress updates while a task runs. It may be called
// from any goroutine and must not block for long.
type ProgressFunc func(Progress)

// Service is a task: a unit of work that runs once, reports progress and
// returns an outcome.
type Service interface {
	resource.Resource
	// Run performs one execution and returns when it is finished. onProgress,
	// when not nil, receives progress as the task reports it. Cancelling ctx
	// cancels the task, which must leave the hardware in a safe state before
	// returning. A second Run while one is in flight returns ErrAlreadyRunning.
	Run(ctx context.Context, input map[string]interface{}, onProgress ProgressFunc, extra map[string]interface{}) (Result, error)
	// GetStatus reports whether the task is running and its last result. It
	// never drives lifecycle.
	GetStatus(ctx context.Context, extra map[string]interface{}) (Status, error)
}

// Named is a helper for getting the named task's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromProvider is a helper for getting the named task from a resource
// Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Service, error) {
	return resource.FromProvider[Service](provider, Named(name))
}

// NamesFromRobot is a helper for getting all task names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// ProgressFromProto converts a proto progress message.
func ProgressFromProto(p *pb.Progress) *Progress {
	if p == nil {
		return nil
	}
	return &Progress{Message: p.Message, Fraction: p.Fraction}
}

// ProgressToProto converts a progress update to its proto message.
func ProgressToProto(p *Progress) *pb.Progress {
	if p == nil {
		return nil
	}
	return &pb.Progress{Message: p.Message, Fraction: p.Fraction}
}

// ResultFromProto converts a proto result message.
func ResultFromProto(r *pb.Result) *Result {
	if r == nil {
		return nil
	}
	return &Result{Outcome: r.Outcome, Output: r.Output.AsMap()}
}
