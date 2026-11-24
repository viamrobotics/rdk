// Package statemachine defines the state machine service: a configured graph
// of states that runs tasks and reacts to events. Each state may run one
// task; when that task returns an outcome, or an event arrives, the machine
// looks up the transition and enters the next state. States nest, tasks emit
// outcomes, and the machine owns transitions.
package statemachine

import (
	"context"
	"time"

	pb "go.viam.com/api/service/statemachine/v1"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/task"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterStateMachineServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.StateMachineService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is the name of the type of service.
const SubtypeName = "state_machine"

// API is a variable that identifies the state machine service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Status is whether the machine is driving its graph.
type Status int

const (
	// StatusRunning means the machine enters states and runs their tasks.
	StatusRunning Status = iota
	// StatusStopped means Stop was called: the machine holds its state and
	// runs nothing until Reset or an event with a transition out.
	StatusStopped
	// StatusFaulted means a task returned an outcome no state handles, or
	// would not cancel in time. The machine runs nothing until Reset or an
	// event with a transition out.
	StatusFaulted
)

// String returns the lower case name of the status.
func (s Status) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusFaulted:
		return "faulted"
	case StatusRunning:
		fallthrough
	default:
		return "running"
	}
}

// Triggers the machine itself records in its history, alongside task
// outcomes and event names.
const (
	// TriggerInitial is recorded when the machine first enters its initial state.
	TriggerInitial = "initial"
	// TriggerStop is recorded when Stop holds the machine.
	TriggerStop = "stop"
	// TriggerReset is recorded when Reset re-enters the initial state.
	TriggerReset = "reset"
	// TriggerFault is recorded when the machine faults.
	TriggerFault = "fault"
)

// TransitionSource is what kind of thing caused a transition.
type TransitionSource int

const (
	// SourceUnspecified is a transition whose cause was not recorded.
	SourceUnspecified TransitionSource = iota
	// SourceInitial is the entry into the initial state at startup.
	SourceInitial
	// SourceOutcome is a task in the from state returning the trigger as its outcome.
	SourceOutcome
	// SourceEvent is SendEvent or a watcher raising the trigger as an event.
	SourceEvent
	// SourceTimeout is the from state's timeout elapsing.
	SourceTimeout
	// SourceStop is Stop holding the machine in place.
	SourceStop
	// SourceReset is Reset re-entering the initial state.
	SourceReset
	// SourceFault is the machine faulting in place.
	SourceFault
)

// String returns the lower case name of the source.
func (s TransitionSource) String() string {
	switch s {
	case SourceInitial:
		return "initial"
	case SourceOutcome:
		return "outcome"
	case SourceEvent:
		return "event"
	case SourceTimeout:
		return "timeout"
	case SourceStop:
		return "stop"
	case SourceReset:
		return "reset"
	case SourceFault:
		return "fault"
	case SourceUnspecified:
		fallthrough
	default:
		return "unspecified"
	}
}

// State is where the machine is right now.
type State struct {
	// Path is the active state path from the root, e.g. ["auto", "pour", "picking_glass"].
	Path   []string
	Status Status
	// Context is the machine's data: configured values merged with task outputs.
	Context map[string]interface{}
	// EnteredAt is when the innermost active state was entered.
	EnteredAt time.Time
	// TaskProgress is the running task's latest progress. Nil when no task is running.
	TaskProgress *task.Progress
	// JobID is the current job, read together with the rest of the state.
	JobID string
}

// EventResult is the outcome of sending an event.
type EventResult struct {
	// Handled is whether a transition matched the event.
	Handled bool
	// Path is the active state path after the event was processed.
	Path []string
}

// Transition is one recorded change of active state.
type Transition struct {
	From, To []string
	// Trigger is what caused it: the task outcome, the event name, or one of the Trigger constants.
	Trigger   string
	Timestamp time.Time
	// TaskResult is the result of the task that was running in From, if it finished.
	TaskResult *task.Result
	// Source is what kind of thing caused the transition. An outcome name sent
	// through SendEvent is recorded as SourceEvent.
	Source TransitionSource
	// JobID is the job the transition belongs to.
	JobID string
}

// StateDefinition is one state of the machine as configured.
type StateDefinition struct {
	// Path is the state's path from the root, e.g. ["auto", "pour", "picking_glass"].
	Path []string
	// Initial is the child entered first, for a compound state. Empty for a leaf.
	Initial string
	// On maps the triggers (task outcomes and event names) the state handles to
	// target state names as written in config, "$history" and "$done" included.
	On map[string]string
	// Task is the task the state runs: a task resource's name, or a built-in
	// task type prefixed with "$" ("$wait_for"). Empty when it runs none.
	Task string
	// Timeout is how long the task may run before the state handles "timeout".
	// Zero when the state has no timeout.
	Timeout time.Duration
}

// Definition is the machine's graph as configured.
type Definition struct {
	// Initial is the top-level state entered on start and on Reset.
	Initial string
	// States is every state, compound states included.
	States []StateDefinition
	// Events is every trigger any state or the machine handles. Task outcomes
	// and external events share this namespace.
	Events []string
	// On maps the triggers the machine handles at its top level to target state names.
	On map[string]string
}

// Service is a state machine.
type Service interface {
	resource.Resource
	// GetState returns the active state path, the status, the context and the running task's progress.
	GetState(ctx context.Context, extra map[string]interface{}) (State, error)
	// SendEvent delivers an external event. An event with no matching transition in the current state
	// or any ancestor is dropped. Events are never queued.
	SendEvent(ctx context.Context, event string, payload, extra map[string]interface{}) (EventResult, error)
	// GetHistory returns the most recent transitions, oldest first. limit <= 0 returns all retained.
	GetHistory(ctx context.Context, limit int, extra map[string]interface{}) ([]Transition, error)
	// GetDefinition returns the graph as configured.
	GetDefinition(ctx context.Context, extra map[string]interface{}) (Definition, error)
	// CurrentJobID identifies the machine's current run. A new job starts each
	// time the machine enters its initial state from the outside: at startup
	// and on Reset.
	CurrentJobID(ctx context.Context, extra map[string]interface{}) (string, error)
	// Stop cancels the current task and holds the machine in its current state.
	Stop(ctx context.Context, extra map[string]interface{}) error
	// Reset cancels the current task, restores the configured context and re-enters the initial state.
	Reset(ctx context.Context, extra map[string]interface{}) error
}

// Named is a helper for getting the named state machine's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromProvider is a helper for getting the named state machine from a
// resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Service, error) {
	return resource.FromProvider[Service](provider, Named(name))
}

// NamesFromRobot is a helper for getting all state machine names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// DefinitionToProto converts a definition to its proto message.
func DefinitionToProto(d Definition) *pb.GetDefinitionResponse {
	resp := &pb.GetDefinitionResponse{Initial: d.Initial, Events: d.Events, On: d.On}
	for _, st := range d.States {
		stPb := &pb.StateDefinition{Path: st.Path, Initial: st.Initial, On: st.On, Task: st.Task}
		if st.Timeout > 0 {
			stPb.Timeout = durationpb.New(st.Timeout)
		}
		resp.States = append(resp.States, stPb)
	}
	return resp
}

// DefinitionFromProto converts a proto definition message.
func DefinitionFromProto(resp *pb.GetDefinitionResponse) Definition {
	d := Definition{Initial: resp.Initial, Events: resp.Events, On: resp.On}
	for _, st := range resp.States {
		d.States = append(d.States, StateDefinition{
			Path: st.Path, Initial: st.Initial, On: st.On, Task: st.Task, Timeout: st.Timeout.AsDuration(),
		})
	}
	return d
}

// SourceToProto converts a transition source to its proto enum.
func SourceToProto(s TransitionSource) pb.TransitionSource {
	switch s {
	case SourceInitial:
		return pb.TransitionSource_TRANSITION_SOURCE_INITIAL
	case SourceOutcome:
		return pb.TransitionSource_TRANSITION_SOURCE_OUTCOME
	case SourceEvent:
		return pb.TransitionSource_TRANSITION_SOURCE_EVENT
	case SourceTimeout:
		return pb.TransitionSource_TRANSITION_SOURCE_TIMEOUT
	case SourceStop:
		return pb.TransitionSource_TRANSITION_SOURCE_STOP
	case SourceReset:
		return pb.TransitionSource_TRANSITION_SOURCE_RESET
	case SourceFault:
		return pb.TransitionSource_TRANSITION_SOURCE_FAULT
	case SourceUnspecified:
		fallthrough
	default:
		return pb.TransitionSource_TRANSITION_SOURCE_UNSPECIFIED
	}
}

// SourceFromProto converts a proto transition source enum.
func SourceFromProto(s pb.TransitionSource) TransitionSource {
	switch s {
	case pb.TransitionSource_TRANSITION_SOURCE_INITIAL:
		return SourceInitial
	case pb.TransitionSource_TRANSITION_SOURCE_OUTCOME:
		return SourceOutcome
	case pb.TransitionSource_TRANSITION_SOURCE_EVENT:
		return SourceEvent
	case pb.TransitionSource_TRANSITION_SOURCE_TIMEOUT:
		return SourceTimeout
	case pb.TransitionSource_TRANSITION_SOURCE_STOP:
		return SourceStop
	case pb.TransitionSource_TRANSITION_SOURCE_RESET:
		return SourceReset
	case pb.TransitionSource_TRANSITION_SOURCE_FAULT:
		return SourceFault
	case pb.TransitionSource_TRANSITION_SOURCE_UNSPECIFIED:
		fallthrough
	default:
		return SourceUnspecified
	}
}

// StatusToProto converts a status to its proto enum.
func StatusToProto(s Status) pb.Status {
	switch s {
	case StatusStopped:
		return pb.Status_STATUS_STOPPED
	case StatusFaulted:
		return pb.Status_STATUS_FAULTED
	case StatusRunning:
		fallthrough
	default:
		return pb.Status_STATUS_RUNNING
	}
}

// StatusFromProto converts a proto status enum.
func StatusFromProto(s pb.Status) Status {
	switch s {
	case pb.Status_STATUS_STOPPED:
		return StatusStopped
	case pb.Status_STATUS_FAULTED:
		return StatusFaulted
	case pb.Status_STATUS_RUNNING, pb.Status_STATUS_UNSPECIFIED:
		fallthrough
	default:
		return StatusRunning
	}
}
