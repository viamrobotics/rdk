package server_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"go.uber.org/zap/zapcore"
	commonpb "go.viam.com/api/common/v1"
	armpb "go.viam.com/api/component/arm/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/cloud"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

var emptyResources = &pb.ResourceNamesResponse{
	Resources: []*commonpb.ResourceName{},
}

var serverNewResource = arm.Named("")

var serverOneResourceResponse = []*commonpb.ResourceName{
	{
		Namespace: string(serverNewResource.API.Type.Namespace),
		Type:      serverNewResource.API.Type.Name,
		Subtype:   serverNewResource.API.SubtypeName,
		Name:      serverNewResource.Name,
	},
}

func TestServer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		server := server.New(injectRobot)

		resourceResp, err := server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp, test.ShouldResemble, emptyResources)

		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{serverNewResource} }

		resourceResp, err = server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp.Resources, test.ShouldResemble, serverOneResourceResponse)
	})

	t.Run("GetMachineStatus", func(t *testing.T) {
		testCases := []struct {
			name                     string
			injectMachineStatus      robot.MachineStatus
			expConfig                *pb.ConfigStatus
			expResources             []*pb.ResourceStatus
			expState                 pb.GetMachineStatusResponse_State
			expBadResourceStateCount int
			expBadMachineStateCount  int
		}{
			{
				"no resources",
				robot.MachineStatus{
					Config:    config.Revision{Revision: "rev1"},
					Resources: []resource.Status{},
					State:     robot.StateRunning,
				},
				&pb.ConfigStatus{Revision: "rev1"},
				[]*pb.ResourceStatus{},
				pb.GetMachineStatusResponse_STATE_RUNNING,
				0,
				0,
			},
			{
				"resource with unknown status",
				robot.MachineStatus{
					Config: config.Revision{Revision: "rev1"},
					Resources: []resource.Status{
						{
							NodeStatus: resource.NodeStatus{
								Name:     arm.Named("badArm"),
								Revision: "rev0",
							},
						},
					},
					State: robot.StateRunning,
				},
				&pb.ConfigStatus{Revision: "rev1"},
				[]*pb.ResourceStatus{
					{
						Name:     protoutils.ResourceNameToProto(arm.Named("badArm")),
						State:    pb.ResourceStatus_STATE_UNSPECIFIED,
						Revision: "rev0",
					},
				},
				pb.GetMachineStatusResponse_STATE_RUNNING,
				1,
				0,
			},
			{
				"resource with valid status",
				robot.MachineStatus{
					Config: config.Revision{Revision: "rev1"},
					Resources: []resource.Status{
						{
							NodeStatus: resource.NodeStatus{
								Name:     arm.Named("goodArm"),
								State:    resource.NodeStateConfiguring,
								Revision: "rev1",
							},
						},
					},
					State: robot.StateRunning,
				},
				&pb.ConfigStatus{Revision: "rev1"},
				[]*pb.ResourceStatus{
					{
						Name:     protoutils.ResourceNameToProto(arm.Named("goodArm")),
						State:    pb.ResourceStatus_STATE_CONFIGURING,
						Revision: "rev1",
					},
				},
				pb.GetMachineStatusResponse_STATE_RUNNING,
				0,
				0,
			},
			{
				"resource with empty cloud metadata",
				robot.MachineStatus{
					Config: config.Revision{Revision: "rev1"},
					Resources: []resource.Status{
						{
							NodeStatus: resource.NodeStatus{
								Name:     arm.Named("goodArm"),
								State:    resource.NodeStateConfiguring,
								Revision: "rev1",
							},
							CloudMetadata: cloud.Metadata{},
						},
					},
					State: robot.StateRunning,
				},
				&pb.ConfigStatus{Revision: "rev1"},
				[]*pb.ResourceStatus{
					{
						Name:          protoutils.ResourceNameToProto(arm.Named("goodArm")),
						State:         pb.ResourceStatus_STATE_CONFIGURING,
						Revision:      "rev1",
						CloudMetadata: &pb.GetCloudMetadataResponse{},
					},
				},
				pb.GetMachineStatusResponse_STATE_RUNNING,
				0,
				0,
			},
			{
				"resource with cloud metadata",
				robot.MachineStatus{
					Config: config.Revision{Revision: "rev1"},
					Resources: []resource.Status{
						{
							NodeStatus: resource.NodeStatus{
								Name:     arm.Named("arm1"),
								State:    resource.NodeStateConfiguring,
								Revision: "rev1",
							},
							CloudMetadata: cloud.Metadata{
								PrimaryOrgID:  "org1",
								LocationID:    "loc1",
								MachineID:     "mac1",
								MachinePartID: "part1",
							},
						},
						{
							NodeStatus: resource.NodeStatus{
								Name:  arm.Named("arm2").PrependRemote("remote1"),
								State: resource.NodeStateReady,
							},
							CloudMetadata: cloud.Metadata{
								PrimaryOrgID:  "org2",
								LocationID:    "loc2",
								MachineID:     "mac2",
								MachinePartID: "part2",
							},
						},
					},
					State: robot.StateRunning,
				},
				&pb.ConfigStatus{Revision: "rev1"},
				[]*pb.ResourceStatus{
					{
						Name:     protoutils.ResourceNameToProto(arm.Named("arm1")),
						State:    pb.ResourceStatus_STATE_CONFIGURING,
						Revision: "rev1",
						CloudMetadata: protoutils.MetadataToProto(
							cloud.Metadata{
								PrimaryOrgID:  "org1",
								LocationID:    "loc1",
								MachineID:     "mac1",
								MachinePartID: "part1",
							},
						),
					},
					{
						Name:  protoutils.ResourceNameToProto(arm.Named("arm2").PrependRemote("remote1")),
						State: pb.ResourceStatus_STATE_READY,
						CloudMetadata: protoutils.MetadataToProto(
							cloud.Metadata{
								PrimaryOrgID:  "org2",
								LocationID:    "loc2",
								MachineID:     "mac2",
								MachinePartID: "part2",
							},
						),
					},
				},
				pb.GetMachineStatusResponse_STATE_RUNNING,
				0,
				0,
			},
			{
				"resources with mixed valid and invalid statuses",
				robot.MachineStatus{
					Config: config.Revision{Revision: "rev1"},
					Resources: []resource.Status{
						{
							NodeStatus: resource.NodeStatus{
								Name:     arm.Named("goodArm"),
								State:    resource.NodeStateConfiguring,
								Revision: "rev1",
							},
						},
						{
							NodeStatus: resource.NodeStatus{
								Name:     arm.Named("badArm"),
								Revision: "rev0",
							},
						},
						{
							NodeStatus: resource.NodeStatus{
								Name:     arm.Named("anotherBadArm"),
								Revision: "rev-1",
							},
						},
					},
					State: robot.StateRunning,
				},
				&pb.ConfigStatus{Revision: "rev1"},
				[]*pb.ResourceStatus{
					{
						Name:     protoutils.ResourceNameToProto(arm.Named("goodArm")),
						State:    pb.ResourceStatus_STATE_CONFIGURING,
						Revision: "rev1",
					},
					{
						Name:     protoutils.ResourceNameToProto(arm.Named("badArm")),
						State:    pb.ResourceStatus_STATE_UNSPECIFIED,
						Revision: "rev0",
					},
					{
						Name:     protoutils.ResourceNameToProto(arm.Named("anotherBadArm")),
						State:    pb.ResourceStatus_STATE_UNSPECIFIED,
						Revision: "rev-1",
					},
				},
				pb.GetMachineStatusResponse_STATE_RUNNING,
				2,
				0,
			},
			{
				"unhealthy status",
				robot.MachineStatus{
					Config: config.Revision{Revision: "rev1"},
					Resources: []resource.Status{
						{
							NodeStatus: resource.NodeStatus{
								Name:     arm.Named("brokenArm"),
								Revision: "rev1",
								State:    resource.NodeStateUnhealthy,
								Error:    errors.New("bad configuration"),
							},
						},
					},
					State: robot.StateRunning,
				},
				&pb.ConfigStatus{Revision: "rev1"},
				[]*pb.ResourceStatus{
					{
						Name:     protoutils.ResourceNameToProto(arm.Named("brokenArm")),
						State:    pb.ResourceStatus_STATE_UNHEALTHY,
						Revision: "rev1",
						Error:    "bad configuration",
					},
				},
				pb.GetMachineStatusResponse_STATE_RUNNING,
				0,
				0,
			},
			{
				"initializing machine state",
				robot.MachineStatus{
					Config:    config.Revision{Revision: "rev1"},
					Resources: []resource.Status{},
					State:     robot.StateInitializing,
				},
				&pb.ConfigStatus{Revision: "rev1"},
				[]*pb.ResourceStatus{},
				pb.GetMachineStatusResponse_STATE_INITIALIZING,
				0,
				0,
			},
			{
				"unknown machine state",
				robot.MachineStatus{
					Config:    config.Revision{Revision: "rev1"},
					Resources: []resource.Status{},
					State:     robot.StateUnknown,
				},
				&pb.ConfigStatus{Revision: "rev1"},
				[]*pb.ResourceStatus{},
				pb.GetMachineStatusResponse_STATE_UNSPECIFIED,
				0,
				1,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				logger, logs := logging.NewObservedTestLogger(t)
				injectRobot := &inject.Robot{}
				server := server.New(injectRobot)
				req := pb.GetMachineStatusRequest{}
				injectRobot.LoggerFunc = func() logging.Logger {
					return logger
				}
				injectRobot.MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
					return tc.injectMachineStatus, nil
				}
				resp, err := server.GetMachineStatus(context.Background(), &req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp.GetConfig().GetRevision(), test.ShouldEqual, tc.expConfig.Revision)
				for i, res := range resp.GetResources() {
					test.That(t, res.GetName(), test.ShouldResemble, tc.expResources[i].Name)
					test.That(t, res.GetState(), test.ShouldResemble, tc.expResources[i].State)
					test.That(t, res.GetRevision(), test.ShouldEqual, tc.expResources[i].Revision)
				}

				test.That(t, resp.GetState(), test.ShouldEqual, tc.expState)

				const badResourceStateMsg = "resource in an unknown state"
				badResourceStateCount := logs.FilterLevelExact(zapcore.ErrorLevel).FilterMessageSnippet(badResourceStateMsg).Len()
				test.That(t, badResourceStateCount, test.ShouldEqual, tc.expBadResourceStateCount)

				const badMachineStateMsg = "machine in an unknown state"
				badMachineStateCount := logs.FilterLevelExact(zapcore.ErrorLevel).FilterMessageSnippet(badMachineStateMsg).Len()
				test.That(t, badMachineStateCount, test.ShouldEqual, tc.expBadMachineStateCount)
			})
		}
	})

	t.Run("GetCloudMetadata", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		req := pb.GetCloudMetadataRequest{}
		injectRobot.CloudMetadataFunc = func(ctx context.Context) (cloud.Metadata, error) {
			return cloud.Metadata{
				PrimaryOrgID:  "the-primary-org",
				LocationID:    "the-location",
				MachineID:     "the-machine",
				MachinePartID: "the-robot-part",
			}, nil
		}
		resp, err := server.GetCloudMetadata(context.Background(), &req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.GetLocationId(), test.ShouldEqual, "the-location")
		test.That(t, resp.GetPrimaryOrgId(), test.ShouldEqual, "the-primary-org")
		test.That(t, resp.GetMachineId(), test.ShouldEqual, "the-machine")
		test.That(t, resp.GetMachinePartId(), test.ShouldEqual, "the-robot-part")
	})

	//nolint:deprecated,staticcheck
	t.Run("Discovery", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		injectRobot.LoggerFunc = func() logging.Logger { return logging.NewTestLogger(t) }
		server := server.New(injectRobot)

		q := resource.DiscoveryQuery{arm.Named("arm").API, resource.DefaultModelFamily.WithModel("some-arm"), nil}
		disc := resource.Discovery{Query: q, Results: struct{}{}}
		discoveries := []resource.Discovery{disc}
		injectRobot.DiscoverComponentsFunc = func(ctx context.Context, keys []resource.DiscoveryQuery) ([]resource.Discovery, error) {
			return discoveries, nil
		}

		t.Run("full api and model", func(t *testing.T) {
			req := &pb.DiscoverComponentsRequest{
				Queries: []*pb.DiscoveryQuery{{Subtype: q.API.String(), Model: q.Model.String()}},
			}

			resp, err := server.DiscoverComponents(context.Background(), req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(resp.Discovery), test.ShouldEqual, 1)

			observed := resp.Discovery[0].Results.AsMap()
			expected := map[string]interface{}{}
			expectedQ := &pb.DiscoveryQuery{Subtype: "rdk:component:arm", Model: "rdk:builtin:some-arm", Extra: &structpb.Struct{}}
			test.That(t, resp.Discovery[0].Query, test.ShouldResemble, expectedQ)
			test.That(t, observed, test.ShouldResemble, expected)
		})
		t.Run("short api and model", func(t *testing.T) {
			req := &pb.DiscoverComponentsRequest{
				Queries: []*pb.DiscoveryQuery{{Subtype: "arm", Model: "some-arm"}},
			}

			resp, err := server.DiscoverComponents(context.Background(), req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(resp.Discovery), test.ShouldEqual, 1)

			observed := resp.Discovery[0].Results.AsMap()
			expected := map[string]interface{}{}
			expectedQ := &pb.DiscoveryQuery{Subtype: "arm", Model: "some-arm", Extra: &structpb.Struct{}}
			test.That(t, resp.Discovery[0].Query, test.ShouldResemble, expectedQ)
			test.That(t, observed, test.ShouldResemble, expected)
		})
	})

	t.Run("ResourceRPCSubtypes", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
		server := server.New(injectRobot)

		typesResp, err := server.ResourceRPCSubtypes(context.Background(), &pb.ResourceRPCSubtypesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, typesResp, test.ShouldResemble, &pb.ResourceRPCSubtypesResponse{
			ResourceRpcSubtypes: []*pb.ResourceRPCSubtype{},
		})

		desc1, err := grpcreflect.LoadServiceDescriptor(&pb.RobotService_ServiceDesc)
		test.That(t, err, test.ShouldBeNil)

		desc2, err := grpcreflect.LoadServiceDescriptor(&armpb.ArmService_ServiceDesc)
		test.That(t, err, test.ShouldBeNil)

		otherAPI := resource.NewAPI("acme", "component", "wat")
		respWith := []resource.RPCAPI{
			{
				API:  serverNewResource.API,
				Desc: desc1,
			},
			{
				API:  resource.NewAPI("acme", "component", "wat"),
				Desc: desc2,
			},
		}

		expectedResp := []*pb.ResourceRPCSubtype{
			{
				Subtype: &commonpb.ResourceName{
					Namespace: string(serverNewResource.API.Type.Namespace),
					Type:      serverNewResource.API.Type.Name,
					Subtype:   serverNewResource.API.SubtypeName,
				},
				ProtoService: desc1.GetFullyQualifiedName(),
			},
			{
				Subtype: &commonpb.ResourceName{
					Namespace: string(otherAPI.Type.Namespace),
					Type:      otherAPI.Type.Name,
					Subtype:   otherAPI.SubtypeName,
				},
				ProtoService: desc2.GetFullyQualifiedName(),
			},
		}

		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return respWith }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }

		typesResp, err = server.ResourceRPCSubtypes(context.Background(), &pb.ResourceRPCSubtypesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, typesResp.ResourceRpcSubtypes, test.ShouldResemble, expectedResp)
	})

	t.Run("GetOperations", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
		injectRobot.LoggerFunc = func() logging.Logger {
			return logger
		}
		server := server.New(injectRobot)

		opsResp, err := server.GetOperations(context.Background(), &pb.GetOperationsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, opsResp, test.ShouldResemble, &pb.GetOperationsResponse{
			Operations: []*pb.Operation{},
		})

		sess1 := session.New(context.Background(), "owner1", time.Minute, nil)
		sess2 := session.New(context.Background(), "owner2", time.Minute, nil)
		sess1Ctx := session.ToContext(context.Background(), sess1)
		sess2Ctx := session.ToContext(context.Background(), sess2)
		op1, cancel1 := injectRobot.OperationManager().Create(context.Background(), "something1", nil)
		defer cancel1()

		op2, cancel2 := injectRobot.OperationManager().Create(sess1Ctx, "something2", nil)
		defer cancel2()

		op3, cancel3 := injectRobot.OperationManager().Create(sess2Ctx, "something3", nil)
		defer cancel3()

		opsResp, err = server.GetOperations(context.Background(), &pb.GetOperationsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, opsResp.Operations, test.ShouldHaveLength, 3)

		for idx, searchFor := range []struct {
			opID   uuid.UUID
			sessID uuid.UUID
		}{
			{operation.Get(op1).ID, uuid.Nil},
			{operation.Get(op2).ID, sess1.ID()},
			{operation.Get(op3).ID, sess2.ID()},
		} {
			t.Run(fmt.Sprintf("check op=%d", idx), func(t *testing.T) {
				for _, op := range opsResp.Operations {
					if op.Id == searchFor.opID.String() {
						if searchFor.sessID == uuid.Nil {
							test.That(t, op.SessionId, test.ShouldBeNil)
							return
						}
						test.That(t, op.SessionId, test.ShouldNotBeNil)
						test.That(t, *op.SessionId, test.ShouldEqual, searchFor.sessID.String())
						return
					}
				}
				t.Fail()
			})
		}
	})

	t.Run("GetSessions", func(t *testing.T) {
		sessMgr := &sessionManager{}
		injectRobot := &inject.Robot{SessMgr: sessMgr}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
		server := server.New(injectRobot)

		sessResp, err := server.GetSessions(context.Background(), &pb.GetSessionsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sessResp, test.ShouldResemble, &pb.GetSessionsResponse{
			Sessions: []*pb.Session{},
		})

		ownerID1 := "owner1"
		remoteAddr1 := &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 5}
		ctx1 := peer.NewContext(context.Background(), &peer.Peer{
			Addr: remoteAddr1,
		})
		remoteAddr2 := &net.TCPAddr{IP: net.IPv4(2, 2, 3, 8), Port: 9}
		ctx2 := peer.NewContext(context.Background(), &peer.Peer{
			Addr: remoteAddr2,
		})

		ownerID2 := "owner2"
		dur := time.Second

		sessions := []*session.Session{
			session.New(ctx1, ownerID1, dur, nil),
			session.New(ctx2, ownerID2, dur, nil),
		}

		sessMgr.mu.Lock()
		sessMgr.sessions = sessions
		sessMgr.mu.Unlock()

		sessResp, err = server.GetSessions(context.Background(), &pb.GetSessionsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sessResp, test.ShouldResemble, &pb.GetSessionsResponse{
			Sessions: []*pb.Session{
				{
					Id:                 sessions[0].ID().String(),
					PeerConnectionInfo: sessions[0].PeerConnectionInfo(),
				},
				{
					Id:                 sessions[1].ID().String(),
					PeerConnectionInfo: sessions[1].PeerConnectionInfo(),
				},
			},
		})
	})

	t.Run("Shutdown", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
		shutdownCalled := false
		injectRobot.ShutdownFunc = func(ctx context.Context) error {
			shutdownCalled = true
			return nil
		}

		server := server.New(injectRobot)
		req := pb.ShutdownRequest{}

		_, err := server.Shutdown(context.Background(), &req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, shutdownCalled, test.ShouldBeTrue)
	})

	t.Run("GetVersion", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		req := pb.GetVersionRequest{}

		server := server.New(injectRobot)
		resp, err := server.GetVersion(context.Background(), &req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.GetPlatform(), test.ShouldEqual, "rdk")
		test.That(t, resp.GetVersion(), test.ShouldEqual, "dev-unknown")
		test.That(t, resp.GetApiVersion(), test.ShouldEqual, "?")
	})
}

func TestServerFrameSystemConfig(t *testing.T) {
	injectRobot := &inject.Robot{}

	o1 := &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
	test.That(t, err, test.ShouldBeNil)

	// test working config function
	t.Run("test working config function", func(t *testing.T) {
		l1 := &referenceframe.LinkConfig{
			ID:          "frame1",
			Parent:      referenceframe.World,
			Translation: r3.Vector{X: 1, Y: 2, Z: 3},
			Orientation: o1Cfg,
			Geometry:    &spatialmath.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
		}
		lif1, err := l1.ParseConfig()
		test.That(t, err, test.ShouldBeNil)
		l2 := &referenceframe.LinkConfig{
			ID:          "frame2",
			Parent:      "frame1",
			Translation: r3.Vector{X: 1, Y: 2, Z: 3},
			Geometry:    &spatialmath.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
		}
		lif2, err := l2.ParseConfig()
		test.That(t, err, test.ShouldBeNil)
		fsConfigs := []*referenceframe.FrameSystemPart{
			{
				FrameConfig: lif1,
			},
			{
				FrameConfig: lif2,
			},
		}

		injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
			return &framesystem.Config{Parts: fsConfigs}, nil
		}
		server := server.New(injectRobot)
		req := &pb.FrameSystemConfigRequest{}
		resp, err := server.FrameSystemConfig(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.FrameSystemConfigs), test.ShouldEqual, len(fsConfigs))
		test.That(t, resp.FrameSystemConfigs[0].Frame.ReferenceFrame, test.ShouldEqual, fsConfigs[0].FrameConfig.Name())
		test.That(
			t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.ReferenceFrame,
			test.ShouldEqual,
			fsConfigs[0].FrameConfig.Parent(),
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.X,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Pose().Point().X,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.Y,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Pose().Point().Y,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.Z,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Pose().Point().Z,
		)
		pose := fsConfigs[0].FrameConfig.Pose()
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.OX,
			test.ShouldAlmostEqual,
			pose.Orientation().OrientationVectorDegrees().OX,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.OY,
			test.ShouldAlmostEqual,
			pose.Orientation().OrientationVectorDegrees().OY,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.OZ,
			test.ShouldAlmostEqual,
			pose.Orientation().OrientationVectorDegrees().OZ,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.Theta,
			test.ShouldAlmostEqual,
			pose.Orientation().OrientationVectorDegrees().Theta,
		)
	})

	t.Run("test failing config function", func(t *testing.T) {
		expectedErr := errors.New("failed to retrieve config")
		injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
			return nil, expectedErr
		}
		req := &pb.FrameSystemConfigRequest{}
		server := server.New(injectRobot)
		resp, err := server.FrameSystemConfig(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, expectedErr)
	})

	injectRobot = &inject.Robot{}

	t.Run("test failing on nonexistent server", func(t *testing.T) {
		req := &pb.FrameSystemConfigRequest{}
		server := server.New(injectRobot)
		test.That(t, func() { server.FrameSystemConfig(context.Background(), req) }, test.ShouldPanic)
	})
}

type sessionManager struct {
	mu       sync.Mutex
	sessions []*session.Session
}

func (mgr *sessionManager) Start(ctx context.Context, ownerID string) (*session.Session, error) {
	panic("unimplemented")
}

func (mgr *sessionManager) All() []*session.Session {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.sessions
}

func (mgr *sessionManager) FindByID(ctx context.Context, id uuid.UUID, ownerID string) (*session.Session, error) {
	panic("unimplemented")
}

func (mgr *sessionManager) AssociateResource(id uuid.UUID, resourceName resource.Name) {
	panic("unimplemented")
}

func (mgr *sessionManager) Close() {
}

func (mgr *sessionManager) ServerInterceptors() session.ServerInterceptors {
	panic("unimplemented")
}
