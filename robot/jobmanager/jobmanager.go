// Package jobmanager handles the logic of the jobmanager, responsible for scheduling and
// keeping track of the "jobs" field of the config.
package jobmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fullstorydev/grpcurl"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

const (
	// componentServiceIndex is an index for the desired component or service within the
	// viam service notation. For example, viam.component.movementsensor.v1.MovementSensorService
	// has movementsensor as its second index in a dot separated slice, which is the resource
	// the job manager will be looking for.
	componentServiceIndex int = 2
)

// JobManager keeps track of the currently scheduled jobs and updates the schedule with
// respect to the "jobs" part of the config.
type JobManager struct {
	scheduler     gocron.Scheduler
	logger        logging.Logger
	getResource   func(resource string) (resource.Resource, error)
	namesToJobIDs map[string]uuid.UUID
	ctx           context.Context
	conn          rpc.ClientConn
	isClosed      bool
	closeMutex    sync.Mutex
}

// New sets up the context and grpcConn that is used in scheduled jobs. The actual
// scheduler is initialized and automatically started. Any jobs added to the config will
// then immediately get scheduled according to their "Schedule" field.
func New(
	robotContext context.Context,
	logger logging.Logger,
	getResource func(string) (resource.Resource, error),
	parentAddr config.ParentSockAddrs,
) (*JobManager, error) {
	jobLogger := logger.Sublogger("job_manager")

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = multierr.Combine(err, scheduler.Shutdown())
		}
	}()

	parentAddr.UnixAddr, err = rutils.CleanWindowsSocketPath(runtime.GOOS, parentAddr.UnixAddr)
	if err != nil {
		return nil, err
	}
	dialAddr := "unix://" + parentAddr.UnixAddr
	if rutils.ViamTCPSockets() {
		dialAddr = parentAddr.TCPAddr
	}
	conn, err := grpc.Dial(robotContext, dialAddr, jobLogger)
	if err != nil {
		return nil, err
	}

	jm := &JobManager{
		logger:        jobLogger,
		scheduler:     scheduler,
		getResource:   getResource,
		namesToJobIDs: make(map[string]uuid.UUID),
		ctx:           robotContext,
		conn:          conn,
	}

	jm.scheduler.Start()
	return jm, nil
}

// Close attempts to close the grpcConn of the job scheduler and shuts it down. It is
// not possible to restart the job scheduler after calling Shutdown().
func (jm *JobManager) Close() error {
	// some tests cause Close() to be called twice, so we need mutex protection here.
	jm.closeMutex.Lock()
	defer jm.closeMutex.Unlock()
	if jm.isClosed {
		return nil
	}
	jm.isClosed = true
	jm.logger.CInfo(jm.ctx, "JobManager is shutting down.")
	utils.UncheckedError(jm.conn.Close())
	return jm.scheduler.Shutdown()
}

// createDescriptorSourceAndgRPCMethod sets up a DescriptorSource for grpc translations
// and sets up parts of the grpc method string that will be invoked later.
func (jm *JobManager) createDescriptorSourceAndgRPCMethod(
	res resource.Resource,
	method string,
) (grpcurl.DescriptorSource, string, string, error) {
	refCtx := metadata.NewOutgoingContext(jm.ctx, nil)
	refClient := grpcreflect.NewClientV1Alpha(refCtx, reflectpb.NewServerReflectionClient(jm.conn))
	// TODO(RSDK-9718)
	// refClient.AllowMissingFileDescriptors()
	reflSource := grpcurl.DescriptorSourceFromServer(jm.ctx, refClient)
	descSource := reflSource
	resourceType := res.Name().API.SubtypeName
	// some subtypes have an underscore in their name, like audio_input, input_controller,
	// or pose_tracker, while their APIs do not - so we have to remove the underscore.
	resourceType = strings.ReplaceAll(resourceType, "_", "")
	services, err := descSource.ListServices()
	if err != nil {
		return nil, "", "", err
	}
	var grpcService string
	for _, srv := range services {
		if strings.Split(srv, ".")[componentServiceIndex] == resourceType {
			grpcService = srv
			break
		}
	}
	if grpcService == "" {
		return nil, "", "", errors.Errorf("could not find a service for type: %s", resourceType)
	}
	return descSource, grpcService, method, nil
}

// createJobFunction returns a function that the job scheduler puts on its queue.
func (jm *JobManager) createJobFunction(jc config.JobConfig) func() {
	jobLogger := jm.logger.Sublogger(jc.Name)
	// To support logging for quick jobs (~ on the seconds schedule), we disable log
	// deduplication for job loggers.
	jobLogger.NeverDeduplicate()
	return func() {
		res, err := jm.getResource(jc.Resource)
		if err != nil {
			jobLogger.CWarnw(jm.ctx, "Could not get resource", "error", err.Error())
			return
		}
		if jc.Method == "DoCommand" {
			jobLogger.CInfo(jm.ctx, "Job triggered")
			response, err := res.DoCommand(jm.ctx, jc.Command)
			if err != nil {
				jobLogger.CWarnw(jm.ctx, "Job failed", "error", err.Error())
			} else {
				jobLogger.CInfow(jm.ctx, "Job succeeded", "response", response)
			}
			return
		}

		descSource, grpcService, grpcMethod, err := jm.createDescriptorSourceAndgRPCMethod(res, jc.Method)
		if err != nil {
			jobLogger.CWarnw(jm.ctx, "grpc setup failed", "error", err)
			return
		}

		gRPCArgument := resource.GetResourceNameOverride(grpcService, grpcMethod)
		argumentMap := map[string]string{
			gRPCArgument: jc.Resource,
		}
		argumentBytes, err := json.Marshal(argumentMap)
		if err != nil {
			jobLogger.CWarnw(jm.ctx, "could not serialize gRPC method arguments", "error", err.Error())
			return
		}
		options := grpcurl.FormatOptions{
			EmitJSONDefaultFields: true,
			IncludeTextSeparator:  true,
			AllowUnknownFields:    true,
		}
		rf, formatter, err := grpcurl.RequestParserAndFormatter(
			grpcurl.Format("json"),
			descSource,
			bytes.NewBuffer(argumentBytes),
			options)
		if err != nil {
			jobLogger.CWarnw(jm.ctx, "could not create parser and formatter for grpc requests", "error", err.Error())
			return
		}

		buffer := bytes.NewBuffer(make([]byte, 0))
		h := &grpcurl.DefaultEventHandler{
			Out:            buffer,
			Formatter:      formatter,
			VerbosityLevel: 0,
		}
		jobLogger.CInfo(jm.ctx, "Job triggered")
		grpcMethodCombined := grpcService + "." + grpcMethod
		err = grpcurl.InvokeRPC(jm.ctx, descSource, jm.conn, grpcMethodCombined, nil, h, rf.Next)
		if err != nil {
			jobLogger.CWarnw(jm.ctx, "Job failed", "error", err.Error())
			return
		} else if h.Status != nil && h.Status.Err() != nil {
			jobLogger.CWarnw(jm.ctx, "Job failed", "error", h.Status.Err())
			return
		}
		response := map[string]any{}
		err = json.Unmarshal(buffer.Bytes(), &response)
		if err != nil {
			jobLogger.CWarnw(jm.ctx, "Unmarshalling grpc response failed with error", "error", err.Error())
		} else {
			jobLogger.CInfow(jm.ctx, "Job succeeded", "response", response)
		}
	}
}

// removeJob removes the job from the scheduler and clears the internal map entry.
func (jm *JobManager) removeJob(name string, verbose bool) {
	jobID := jm.namesToJobIDs[name]
	if verbose {
		jm.logger.CInfow(jm.ctx, "Removing job", "name", name)
	}
	err := jm.scheduler.RemoveJob(jobID)
	if err != nil {
		jm.logger.CWarnw(jm.ctx, "Removing the job failed", "error", err.Error())
	}
	delete(jm.namesToJobIDs, name)
}

// scheduleJob validates the job config and attempts to put a new job on the scheduler
// queue. If an error happens, it is logged, and the job is not scheduled.
func (jm *JobManager) scheduleJob(jc config.JobConfig, verbose bool) {
	if err := jc.Validate(""); err != nil {
		jm.logger.CWarnw(jm.ctx, "Job failed to validate", "name", jc.Name, "error", err.Error())
		return
	}

	var jobType gocron.JobDefinition
	var jobLimitMode gocron.LimitMode
	t, err := time.ParseDuration(jc.Schedule)
	if err != nil {
		withSeconds := len(strings.Split(jc.Schedule, " ")) >= 6
		jobType = gocron.CronJob(jc.Schedule, withSeconds)
		jobLimitMode = gocron.LimitModeReschedule
	} else {
		jobType = gocron.DurationJob(t)
		jobLimitMode = gocron.LimitModeWait
	}

	jobFunc := jm.createJobFunction(jc)
	j, err := jm.scheduler.NewJob(
		jobType,
		gocron.NewTask(jobFunc),
		// WithSingletonMode option allows us to perform jobs on the same schedule
		// sequentially. This will guarantee that there is only one instance of a particular
		// job running at the same time. If a job reaches its schedule while the previous
		// iteration is running, the job scheduler will treat them differently based on
		// jobLimitMode.
		// If the job is a CRON job, the run will be skipped if a previous
		// invocation of a job is still running.
		// If the job is a DURATION job, the new job will run as soon as the previous one
		// finishes. This has no effect on the schedule (timer) of the job.

		// Examples:
		// CRON job with a */5 * * * * * (every 5 seconds) timer and a 6s sleep as a function
		// second 0: job starts
		// second 5: job is asleep, the next iteration would be at second 10. Rescheduled
		// second 6: job wakes up, finishes
		// second 10: new iteration (2nd) of the job starts

		// DURATION job with a 5s timer; the first time the function takes 6 seconds, the
		// second takes 3 seconds.
		// second 0: job starts
		// second 5: job is asleep, the new job is put on the queue
		// second 6: job wakes up, finishes. The new job (2nd) starts right now.
		// second 9: the new job is finished. Nothing is on the queue.
		// second 10: another (3rd) iteration of the job starts, based on the original 5s
		// timer.

		// It is also important to note that DURATION jobs start relative to when they were
		// queued on the job scheduler, while CRON jobs are tied to the physical clock.
		gocron.WithSingletonMode(jobLimitMode),
	)

	jobLogger := jm.logger.Sublogger(jc.Name)

	if err != nil {
		jobLogger.CErrorw(jm.ctx, "Failed to create a new job", "name", jc.Name, "error", err.Error())
		return
	}
	jobID := j.ID()

	if verbose {
		jobLogger.CInfow(jm.ctx, "Job created", "name", jc.Name)
	}

	jm.namesToJobIDs[jc.Name] = jobID
}

// UpdateJobs is called when the "jobs" part of the config gets updated. It updates
// scheduled jobs based on the Removed/Added/Modified parts of the diff.
func (jm *JobManager) UpdateJobs(diff *config.Diff) {
	for _, jc := range diff.Removed.Jobs {
		jm.removeJob(jc.Name, true)
	}
	for _, jc := range diff.Modified.Jobs {
		jm.logger.CInfow(jm.ctx, "Job modified", "name", jc.Name)
		jm.removeJob(jc.Name, false)
		jm.scheduleJob(jc, false)
	}
	for _, jc := range diff.Added.Jobs {
		jm.scheduleJob(jc, true)
	}
}
