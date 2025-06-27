package jobmanager

import (
	//"fmt"
	"context"
	"os"
	"strings"
	"time"

	"github.com/fullstorydev/grpcurl"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"

	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils/rpc"
)

// The Jobmanager should:
// - have a concept of a "Job"
// - add or remove "Jobs" according to mutations of jobConfigs
// - log when jobs trigger and whether they succeed or fail
// - gracefully shutdown
// - should have tests.
type Jobmanager struct {
	scheduler    gocron.Scheduler
	jobConfigs   []config.JobConfig
	logger       logging.Logger
	getResource  func(resource string) (resource.Resource, error)
	parentAddr   config.ParentSockAddrs
	namesToUUIDs map[string]uuid.UUID
}

type Job struct {
	// things to consider:
	// functions and their arguments
	// return value of functions
	// logging
	// context
}

// context needed?
func New(jobConfigs []config.JobConfig,
	logger logging.Logger,
	getResource func(string) (resource.Resource, error),
	parentAddr config.ParentSockAddrs,
) (*Jobmanager, error) {
	jobLogger := logger.Sublogger("job_manager")
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	jm := &Jobmanager{
		jobConfigs:   jobConfigs,
		logger:       jobLogger,
		scheduler:    scheduler,
		getResource:  getResource,
		parentAddr:   parentAddr,
		namesToUUIDs: make(map[string]uuid.UUID),
	}
	// want as a go routine?
	//go jm.processJobs()
	return jm, nil
}

func (jm *Jobmanager) Start() {
	jm.scheduler.Start()
}

func (jm *Jobmanager) Stop() error {
	return jm.scheduler.StopJobs()
}

func (jm *Jobmanager) Shutdown() error {
	jm.logger.Info("Shutting down gracefully")
	return jm.scheduler.Shutdown()
}

// fineTune what needs to be here
// return type could be the Job struct?
func (jm *Jobmanager) jobTemplate(jc config.JobConfig, res resource.Resource) (any, []any) {
	// NOTE: with this approach the context deadline must be at least as long as the job
	// interval
	ctxTimeout, cancelFunc := context.WithTimeout(context.Background(), 15*time.Second)
	// how to handle context well?
	_ = cancelFunc
	_ = ctxTimeout
	if jc.Method == "DoCommand" {
		functionToRun := func() {
			jm.logger.Info("triggering the job: ", jc.Name, "at time: ", time.Now())
			result, err := res.DoCommand(context.Background(), jc.Command)
			if err != nil {
				jm.logger.Error("DoCommand for job: ", jc.Name, " failed with error: ", err)
			} else {
				jm.logger.Info("DoCommand for job: ", jc.Name, " succeeded with value: ", result)
			}
		}
		return functionToRun, nil
	}

	err := module.CheckSocketOwner(jm.parentAddr.UnixAddr)
	if err != nil {
		jm.logger.Error(err)
		return nil, nil
	}

	functionToRun := func() {

		ctxTimeout, cancelFunc := context.WithTimeout(context.Background(), 15*time.Second)
		_ = cancelFunc
		// TODO: might need to wrap it around in a function, to close the connection
		dialAddr := "unix://" + jm.parentAddr.UnixAddr
		conn, err := grpc.Dial(ctxTimeout, dialAddr, jm.logger, rpc.WithDialDebug())

		if err != nil {
			jm.logger.Error(err)
		}
		jm.logger.Info("BOG: got here!")

		refCtx := metadata.NewOutgoingContext(ctxTimeout, nil)
		refClient := grpcreflect.NewClientV1Alpha(refCtx, reflectpb.NewServerReflectionClient(conn))
		reflSource := grpcurl.DescriptorSourceFromServer(ctxTimeout, refClient)
		descSource := reflSource

		jm.logger.Info(descSource.ListServices())

		grpcMethod := "viam.component.motor.v1.MotorService." + jc.Method

		data := "{\"name\" : \"motor1\"}" // where arguments are
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

		h := &grpcurl.DefaultEventHandler{
			Out:            os.Stdout,
			Formatter:      formatter,
			VerbosityLevel: 0,
		}

		if err != nil {
			jm.logger.Error(err)
		}

		err = grpcurl.InvokeRPC(ctxTimeout, descSource, conn, grpcMethod, nil, h, rf.Next)

		if err != nil {
			jm.logger.Error(err)
		}
	}

	return functionToRun, nil
}

// this should start the first jobs from the config.
func (jm *Jobmanager) processJobs() {
	// for each job in the config, create the job
	for _, jc := range jm.jobConfigs {
		var jobType gocron.JobDefinition
		t, err := time.ParseDuration(jc.Schedule)
		if err != nil {
			jobType = gocron.CronJob(jc.Schedule, false)
		} else {
			jobType = gocron.DurationJob(t)
		}

		resource, err := jm.getResource(jc.Resource)
		if err != nil {
			jm.logger.Error(err)
			continue
		}
		jobFunc, jobArgs := jm.jobTemplate(jc, resource)
		j, err := jm.scheduler.NewJob(
			jobType,
			gocron.NewTask(
				jobFunc,
				jobArgs...,
			),
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
		)
		if err != nil {
			jm.logger.Error(err)
			continue
		}

		jm.logger.Info("created a job with uuid: ", j.ID())
		jm.namesToUUIDs[jc.Name] = j.ID()
	}

	// want here or separately?
	jm.scheduler.Start()
}

// This should be called when config gets changed.
func (jm *Jobmanager) UpdateJobs(jobs []config.JobConfig) {
	// TODO: definitely wantto make something smarter for jobs that did not change
	// use the diff strategy to track added/removed/modified?
	jm.scheduler.StopJobs()

	for _, uuid := range jm.namesToUUIDs {
		jm.scheduler.RemoveJob(uuid)
	}
	jm.namesToUUIDs = make(map[string]uuid.UUID)
	jm.jobConfigs = jobs
	jm.processJobs()
}
