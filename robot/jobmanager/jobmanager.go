// Package jobmanager handles the logic of the jobmanager, responsible for scheduling and
// keeping track of the "jobs" field of the config.
package jobmanager

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fullstorydev/grpcurl"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// Jobmanager keeps track of the currently scheduled jobs and updates the schedule with
// respect to the "jobs" part of the config.
type Jobmanager struct {
	scheduler    gocron.Scheduler
	logger       logging.Logger
	getResource  func(resource string) (resource.Resource, error)
	namesToUUIDs map[string]uuid.UUID
	ctx          context.Context
	conn         rpc.ClientConn
}

// New sets up the context and grpcConn that is used in scheduled jobs. The actual
// scheduler is initialized and automatically started. Any jobs added to the config will
// then immediately get scheduled according to their "Schedule" field.
func New(
	robotContext context.Context,
	logger logging.Logger,
	getResource func(string) (resource.Resource, error),
	parentAddr config.ParentSockAddrs,
) (*Jobmanager, error) {
	jobLogger := logger.Sublogger("job_manager")
	// we do not want deduplication on jobs
	jobLogger.NeverDeduplicate()
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	dialAddr := "unix://" + parentAddr.UnixAddr
	conn, err := grpc.Dial(robotContext, dialAddr, jobLogger)
	if err != nil {
		return nil, err
	}

	jm := &Jobmanager{
		logger:       jobLogger,
		scheduler:    scheduler,
		getResource:  getResource,
		namesToUUIDs: make(map[string]uuid.UUID),
		ctx:          robotContext,
		conn:         conn,
	}

	jm.scheduler.Start()
	return jm, nil
}

// Shutdown attempts to close the grpcConn of the job scheduler and shuts it down. It is
// not possible to restart the job scheduler after calling Shutdown().
func (jm *Jobmanager) Shutdown() error {
	jm.logger.CInfo(jm.ctx, "Jobmanager is shutting down")
	utils.UncheckedError(jm.conn.Close())
	return jm.scheduler.Shutdown()
}

// createJobFunction returns a function that the job scheduler puts on its queue.
func (jm *Jobmanager) createJobFunction(jc config.JobConfig) func() {
	if jc.Method == "DoCommand" {
		functionToRun := func() {
			resource, err := jm.getResource(jc.Resource)
			if err != nil {
				jm.logger.CWarnw(jm.ctx, "Could not get resource", "error", err)
				return
			}
			jm.logger.CInfow(jm.ctx, "Job triggered", "name", jc.Name)
			result, err := resource.DoCommand(jm.ctx, jc.Command)
			if err != nil {
				jm.logger.CWarnw(jm.ctx, "Job failed", "name", jc.Name, "error", err)
			} else {
				jm.logger.CInfow(jm.ctx, "Job succeeded", "name", jc.Name, "result", result)
			}
		}
		return functionToRun
	}

	functionToRun := func() {
		resource, err := jm.getResource(jc.Resource)
		if err != nil {
			jm.logger.CWarnw(jm.ctx, "Could not get resource", "error", err)
			return
		}
		refCtx := metadata.NewOutgoingContext(jm.ctx, nil)
		refClient := grpcreflect.NewClientV1Alpha(refCtx, reflectpb.NewServerReflectionClient(jm.conn))
		reflSource := grpcurl.DescriptorSourceFromServer(jm.ctx, refClient)
		descSource := reflSource

		resourceType := resource.Name().API.SubtypeName
		services, err := descSource.ListServices()
		if err != nil {
			jm.logger.CWarnw(jm.ctx, "Could not get a list of available grpc services", "error", err)
			return
		}
		var grpcService string
		for _, srv := range services {
			if strings.Contains(srv, resourceType) {
				grpcService = srv
				break
			}
		}
		if grpcService == "" {
			jm.logger.CWarn(jm.ctx, fmt.Sprintf("could not find a service for type: %s", resourceType))
			return
		}
		grpcMethod := grpcService + "." + jc.Method

		data := fmt.Sprintf("{%q : %q}", "name", jc.Resource)
		options := grpcurl.FormatOptions{
			EmitJSONDefaultFields: true,
			IncludeTextSeparator:  true,
			AllowUnknownFields:    true,
		}
		rf, formatter, err := grpcurl.RequestParserAndFormatter(
			grpcurl.Format("json"),
			descSource,
			strings.NewReader(data),
			options)

		buffer := bytes.NewBuffer(make([]byte, 0))
		h := &grpcurl.DefaultEventHandler{
			Out:            buffer,
			Formatter:      formatter,
			VerbosityLevel: 0,
		}

		if err != nil {
			jm.logger.Error(err)
		}

		jm.logger.CInfow(jm.ctx, "Job triggered", "name", jc.Name)
		err = grpcurl.InvokeRPC(jm.ctx, descSource, jm.conn, grpcMethod, nil, h, rf.Next)

		if err != nil {
			jm.logger.CWarnw(jm.ctx, "Job failed", "name", jc.Name, "error", err)
		} else {
			// the output is in JSON, which we want to translate to our logger. So, we manually
			// remove newlines and extra quotes.
			toPrint := buffer.String()
			toPrint = strings.ReplaceAll(toPrint, "\n", "")
			toPrint = strings.ReplaceAll(toPrint, "\"", "")
			jm.logger.CInfow(jm.ctx, "Job succeeded", "name", jc.Name, "result", toPrint)
		}
	}

	return functionToRun
}

// removeJob removes the job from the scheduler and clears the internal map entry.
func (jm *Jobmanager) removeJob(name string) {
	jobID := jm.namesToUUIDs[name]
	err := jm.scheduler.RemoveJob(jobID)
	if err != nil {
		jm.logger.CWarnw(jm.ctx, "Removing the job failed", "error", err)
	}
	delete(jm.namesToUUIDs, name)
}

// scheduleJob validates the job config and attempts to put a new job on the scheduler
// queue. If an error happens, it is logged, and the job is not scheduled.
func (jm *Jobmanager) scheduleJob(jc config.JobConfig) {
	if err := jc.Validate(""); err != nil {
		jm.logger.CWarnw(jm.ctx, "Job failed to validate", "name", jc.Name, "error", err)
		return
	}
	var jobType gocron.JobDefinition
	t, err := time.ParseDuration(jc.Schedule)
	if err != nil {
		withSeconds := len(strings.Split(jc.Schedule, " ")) >= 6
		jobType = gocron.CronJob(jc.Schedule, withSeconds)
	} else {
		jobType = gocron.DurationJob(t)
	}

	jobFunc := jm.createJobFunction(jc)
	j, err := jm.scheduler.NewJob(
		jobType,
		gocron.NewTask(jobFunc),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		jm.logger.CErrorw(jm.ctx, "Failed to create a new job", "name", jc.Name, "error", err)
		return
	}
	jobID := j.ID()
	jm.logger.CInfow(jm.ctx, "Job created", "name", jc.Name, "id", jobID)
	jm.namesToUUIDs[jc.Name] = jobID
}

// UpdateJobs is called when the "jobs" part of the config gets updated. It updates
// scheduled jobs based on the Removed/Added/Modified parts of the diff.
func (jm *Jobmanager) UpdateJobs(diff *config.Diff) {
	for _, jc := range diff.Removed.Jobs {
		jm.removeJob(jc.Name)
	}
	for _, jc := range diff.Modified.Jobs {
		jm.removeJob(jc.Name)
		jm.scheduleJob(jc)
	}
	for _, jc := range diff.Added.Jobs {
		jm.scheduleJob(jc)
	}
}
