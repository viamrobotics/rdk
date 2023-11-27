package config

import (
	"reflect"
	"strings"
	"syscall"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	packagespb "go.viam.com/api/app/packages/v1"
	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	spatial "go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

// FromProto converts the RobotConfig to the internal rdk equivalent.
func FromProto(proto *pb.RobotConfig, logger logging.Logger) (*Config, error) {
	cfg := Config{}

	var err error
	cfg.Cloud, err = CloudConfigFromProto(proto.Cloud)
	if err != nil {
		return nil, errors.Wrap(err, "error converting cloud config from proto")
	}

	if proto.Network != nil {
		network, err := NetworkConfigFromProto(proto.Network)
		if err != nil {
			return nil, errors.Wrap(err, "error converting network config from proto")
		}
		cfg.Network = *network
	}

	if proto.Auth != nil {
		auth, err := AuthConfigFromProto(proto.Auth)
		if err != nil {
			return nil, errors.Wrap(err, "error converting auth config from proto")
		}
		cfg.Auth = *auth
	}
	disablePartialStart := false
	if proto.DisablePartialStart != nil {
		disablePartialStart = *proto.DisablePartialStart
	}

	cfg.Modules, err = toRDKSlice(proto.Modules, ModuleConfigFromProto, disablePartialStart, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error converting modules config from proto")
	}

	cfg.Components, err = toRDKSlice(proto.Components, ComponentConfigFromProto, disablePartialStart, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error converting components config from proto")
	}

	cfg.Remotes, err = toRDKSlice(proto.Remotes, RemoteConfigFromProto, disablePartialStart, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error converting remotes config from proto")
	}

	cfg.Processes, err = toRDKSlice(proto.Processes, ProcessConfigFromProto, disablePartialStart, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error converting processes config from proto")
	}

	cfg.Services, err = toRDKSlice(proto.Services, ServiceConfigFromProto, disablePartialStart, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error converting services config from proto")
	}

	cfg.Packages, err = toRDKSlice(proto.Packages, PackageConfigFromProto, disablePartialStart, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error converting packages config from proto")
	}

	if proto.Debug != nil {
		cfg.Debug = *proto.Debug
	}

	return &cfg, nil
}

// ComponentConfigToProto converts Component to the proto equivalent.
// Assumes config is valid except for partial names which will be completed.
func ComponentConfigToProto(conf *resource.Config) (*pb.ComponentConfig, error) {
	conf.AdjustPartialNames(resource.APITypeComponentName)

	attributes, err := protoutils.StructToStructPb(conf.Attributes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert attributes configs")
	}

	serviceConfigs, err := mapSliceWithErrors(conf.AssociatedResourceConfigs, AssociatedResourceConfigToProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	protoConf := pb.ComponentConfig{
		Name:             conf.Name,
		Namespace:        string(conf.API.Type.Namespace),
		Type:             conf.API.SubtypeName,
		Api:              conf.API.String(),
		Model:            conf.Model.String(),
		DependsOn:        conf.DependsOn,
		ServiceConfigs:   serviceConfigs,
		Attributes:       attributes,
		LogConfiguration: &pb.LogConfiguration{Level: strings.ToLower(conf.LogConfiguration.Level.String())},
	}

	if conf.Frame != nil {
		frame, err := FrameConfigToProto(*conf.Frame)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert frame to proto config")
		}
		protoConf.Frame = frame
	}

	return &protoConf, nil
}

// ComponentConfigFromProto creates Component from the proto equivalent.
func ComponentConfigFromProto(protoConf *pb.ComponentConfig) (*resource.Config, error) {
	serviceConfigs, err := mapSliceWithErrors(protoConf.ServiceConfigs, AssociatedResourceConfigFromProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	// for consistency, nil out empty maps and configs (otherwise go>proto>go conversion doesn't match)
	attrs := protoConf.GetAttributes().AsMap()
	if len(attrs) == 0 {
		attrs = nil
	}

	if len(serviceConfigs) == 0 {
		serviceConfigs = nil
	}

	api, err := resource.NewAPIFromString(protoConf.GetApi())
	if err != nil {
		return nil, err
	}

	model, err := resource.NewModelFromString(protoConf.GetModel())
	if err != nil {
		return nil, err
	}

	level := logging.INFO
	if protoConf.GetLogConfiguration() != nil {
		if level, err = logging.LevelFromString(protoConf.GetLogConfiguration().Level); err != nil {
			// Don't fail configuration due to a malformed log level.
			level = logging.INFO
			logging.Global().Warnw(
				"Invalid log level.", "name", protoConf.GetName(), "log_level", protoConf.GetLogConfiguration().Level)
		}
	}
	componentConf := resource.Config{
		Name:                      protoConf.GetName(),
		API:                       api,
		Model:                     model,
		Attributes:                attrs,
		DependsOn:                 protoConf.GetDependsOn(),
		AssociatedResourceConfigs: serviceConfigs,
		LogConfiguration:          resource.LogConfig{Level: level},
	}

	if protoConf.GetFrame() != nil {
		frame, err := FrameConfigFromProto(protoConf.GetFrame())
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert frame from proto config")
		}
		componentConf.Frame = frame
	}

	return &componentConf, nil
}

// ServiceConfigToProto converts Service to the proto equivalent.
// Assumes config is valid except for partial names which will be completed.
func ServiceConfigToProto(conf *resource.Config) (*pb.ServiceConfig, error) {
	conf.AdjustPartialNames(resource.APITypeServiceName)

	attributes, err := protoutils.StructToStructPb(conf.Attributes)
	if err != nil {
		return nil, err
	}

	serviceConfigs, err := mapSliceWithErrors(conf.AssociatedResourceConfigs, AssociatedResourceConfigToProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	protoConf := pb.ServiceConfig{
		Name:           conf.Name,
		Namespace:      string(conf.API.Type.Namespace),
		Type:           conf.API.SubtypeName,
		Api:            conf.API.String(),
		Model:          conf.Model.String(),
		Attributes:     attributes,
		DependsOn:      conf.DependsOn,
		ServiceConfigs: serviceConfigs,
	}

	return &protoConf, nil
}

// ServiceConfigFromProto creates Service from the proto equivalent shared with Components.
func ServiceConfigFromProto(protoConf *pb.ServiceConfig) (*resource.Config, error) {
	// for consistency, nil out empty map (otherwise go>proto>go conversion doesn't match)
	attrs := protoConf.GetAttributes().AsMap()
	if len(attrs) == 0 {
		attrs = nil
	}

	serviceConfigs, err := mapSliceWithErrors(protoConf.ServiceConfigs, AssociatedResourceConfigFromProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	api, err := resource.NewAPIFromString(protoConf.GetApi())
	if err != nil {
		return nil, err
	}

	model, err := resource.NewModelFromString(protoConf.GetModel())
	if err != nil {
		return nil, err
	}

	conf := resource.Config{
		Name:                      protoConf.GetName(),
		API:                       api,
		Model:                     model,
		Attributes:                attrs,
		DependsOn:                 protoConf.GetDependsOn(),
		AssociatedResourceConfigs: serviceConfigs,
	}

	return &conf, nil
}

// ModuleConfigToProto converts Module to the proto equivalent.
func ModuleConfigToProto(module *Module) (*pb.ModuleConfig, error) {
	var status *pb.AppValidationStatus
	if module.Status != nil {
		status = &pb.AppValidationStatus{Error: module.Status.Error}
	}

	proto := pb.ModuleConfig{
		Name:     module.Name,
		Path:     module.ExePath,
		LogLevel: module.LogLevel,
		Type:     string(module.Type),
		ModuleId: module.ModuleID,
		Env:      module.Environment,
		Status:   status,
	}

	return &proto, nil
}

// ModuleConfigFromProto creates Module from the proto equivalent.
func ModuleConfigFromProto(proto *pb.ModuleConfig) (*Module, error) {
	var status *AppValidationStatus
	if proto.GetStatus() != nil {
		status = &AppValidationStatus{Error: proto.GetStatus().GetError()}
	}

	module := Module{
		Name:        proto.GetName(),
		ExePath:     proto.GetPath(),
		LogLevel:    proto.GetLogLevel(),
		Type:        ModuleType(proto.GetType()),
		ModuleID:    proto.GetModuleId(),
		Environment: proto.GetEnv(),
		Status:      status,
	}
	return &module, nil
}

// ProcessConfigToProto converts ProcessConfig to proto equivalent.
func ProcessConfigToProto(process *pexec.ProcessConfig) (*pb.ProcessConfig, error) {
	return &pb.ProcessConfig{
		Id:          process.ID,
		Name:        process.Name,
		Args:        process.Args,
		Cwd:         process.CWD,
		OneShot:     process.OneShot,
		Env:         process.Environment,
		Log:         process.Log,
		StopSignal:  int32(process.StopSignal),
		StopTimeout: durationpb.New(process.StopTimeout),
	}, nil
}

// ProcessConfigFromProto creates ProcessConfig from the proto equivalent.
func ProcessConfigFromProto(proto *pb.ProcessConfig) (*pexec.ProcessConfig, error) {
	return &pexec.ProcessConfig{
		ID:          proto.Id,
		Name:        proto.Name,
		Args:        proto.Args,
		CWD:         proto.Cwd,
		Environment: proto.Env,
		OneShot:     proto.OneShot,
		Log:         proto.Log,
		StopSignal:  syscall.Signal(proto.StopSignal),
		StopTimeout: proto.StopTimeout.AsDuration(),
	}, nil
}

// AssociatedResourceConfigToProto converts AssociatedResourceConfig to the proto equivalent.
func AssociatedResourceConfigToProto(conf resource.AssociatedResourceConfig) (*pb.ResourceLevelServiceConfig, error) {
	attributes, err := protoutils.StructToStructPb(conf.Attributes)
	if err != nil {
		return nil, err
	}

	proto := pb.ResourceLevelServiceConfig{
		Type:       conf.API.String(),
		Attributes: attributes,
	}

	return &proto, nil
}

// AssociatedResourceConfigFromProto creates AssociatedResourceConfig from the proto equivalent.
func AssociatedResourceConfigFromProto(proto *pb.ResourceLevelServiceConfig) (resource.AssociatedResourceConfig, error) {
	api, err := resource.NewPossibleRDKServiceAPIFromString(proto.GetType())
	if err != nil {
		return resource.AssociatedResourceConfig{}, err
	}

	service := resource.AssociatedResourceConfig{
		API:        api,
		Attributes: proto.GetAttributes().AsMap(),
	}

	return service, nil
}

// FrameConfigToProto converts Frame to the proto equivalent.
func FrameConfigToProto(frame referenceframe.LinkConfig) (*pb.Frame, error) {
	pose, err := frame.Pose()
	if err != nil {
		return nil, err
	}
	pt := pose.Point()
	orient := pose.Orientation()
	proto := pb.Frame{
		Parent: frame.Parent,
		Translation: &pb.Translation{
			X: pt.X,
			Y: pt.Y,
			Z: pt.Z,
		},
	}

	var orientation pb.Orientation

	switch oType := orient.(type) {
	case *spatial.R4AA:
		r4aa := orient.(*spatial.R4AA)
		config := pb.Orientation_AxisAngles{
			Theta: r4aa.Theta,
			X:     r4aa.RX,
			Y:     r4aa.RY,
			Z:     r4aa.RZ,
		}
		orientation = pb.Orientation{
			Type: &pb.Orientation_AxisAngles_{AxisAngles: &config},
		}
	case *spatial.OrientationVector:
		vector := orient.(*spatial.OrientationVector)
		config := pb.Orientation_OrientationVectorRadians{
			Theta: vector.Theta,
			X:     vector.OX,
			Y:     vector.OY,
			Z:     vector.OZ,
		}
		orientation = pb.Orientation{
			Type: &pb.Orientation_VectorRadians{VectorRadians: &config},
		}
	case *spatial.OrientationVectorDegrees:
		vector := orient.(*spatial.OrientationVectorDegrees)
		config := pb.Orientation_OrientationVectorDegrees{
			Theta: vector.Theta,
			X:     vector.OX,
			Y:     vector.OY,
			Z:     vector.OZ,
		}
		orientation = pb.Orientation{
			Type: &pb.Orientation_VectorDegrees{VectorDegrees: &config},
		}
	case *spatial.EulerAngles:
		eulerAngles := orient.(*spatial.EulerAngles)
		config := pb.Orientation_EulerAngles{
			Roll:  eulerAngles.Roll,
			Pitch: eulerAngles.Pitch,
			Yaw:   eulerAngles.Yaw,
		}
		orientation = pb.Orientation{
			Type: &pb.Orientation_EulerAngles_{EulerAngles: &config},
		}
	case *spatial.Quaternion:
		q := orient.(*spatial.Quaternion)
		config := pb.Orientation_Quaternion{
			W: q.Real,
			X: q.Imag,
			Y: q.Jmag,
			Z: q.Kmag,
		}
		orientation = pb.Orientation{
			Type: &pb.Orientation_Quaternion_{Quaternion: &config},
		}
	default:
		return nil, errors.Errorf("Orientation type %s unsupported in json configuration", oType)
	}

	proto.Orientation = &orientation
	if frame.Geometry != nil {
		proto.Geometry, err = frame.Geometry.ToProtobuf()
		if err != nil {
			return nil, err
		}
	}

	return &proto, nil
}

// FrameConfigFromProto creates Frame from the proto equivalent.
func FrameConfigFromProto(proto *pb.Frame) (*referenceframe.LinkConfig, error) {
	var err error
	frame := &referenceframe.LinkConfig{
		Parent: proto.GetParent(),
		Translation: r3.Vector{
			X: proto.GetTranslation().GetX(),
			Y: proto.GetTranslation().GetY(),
			Z: proto.GetTranslation().GetZ(),
		},
	}

	if proto.GetOrientation() != nil {
		var orient spatial.Orientation
		switch or := proto.GetOrientation().Type.(type) {
		case *pb.Orientation_NoOrientation_:
			orient = spatial.NewZeroOrientation()
		case *pb.Orientation_VectorRadians:
			orient = &spatial.OrientationVector{
				Theta: or.VectorRadians.Theta,
				OX:    or.VectorRadians.X,
				OY:    or.VectorRadians.Y,
				OZ:    or.VectorRadians.Z,
			}
		case *pb.Orientation_VectorDegrees:
			orient = &spatial.OrientationVectorDegrees{
				Theta: or.VectorDegrees.Theta,
				OX:    or.VectorDegrees.X,
				OY:    or.VectorDegrees.Y,
				OZ:    or.VectorDegrees.Z,
			}
		case *pb.Orientation_EulerAngles_:
			orient = &spatial.EulerAngles{
				Pitch: or.EulerAngles.Pitch,
				Roll:  or.EulerAngles.Roll,
				Yaw:   or.EulerAngles.Yaw,
			}
		case *pb.Orientation_AxisAngles_:
			orient = &spatial.R4AA{
				Theta: or.AxisAngles.Theta,
				RX:    or.AxisAngles.X,
				RY:    or.AxisAngles.Y,
				RZ:    or.AxisAngles.Z,
			}
		case *pb.Orientation_Quaternion_:
			orient = &spatial.Quaternion{
				Real: or.Quaternion.W,
				Imag: or.Quaternion.X,
				Jmag: or.Quaternion.Y,
				Kmag: or.Quaternion.Z,
			}
		default:
			return nil, errors.New("Orientation type unsupported")
		}
		frame.Orientation, err = spatial.NewOrientationConfig(orient)
		if err != nil {
			return nil, err
		}
	}

	if proto.GetGeometry() != nil {
		geom, err := spatial.NewGeometryFromProto(proto.GetGeometry())
		if err != nil {
			return nil, err
		}
		frame.Geometry, err = spatial.NewGeometryConfig(geom)
		if err != nil {
			return nil, err
		}
	}

	return frame, nil
}

// RemoteConfigToProto converts Remote to the proto equivalent.
func RemoteConfigToProto(remote *Remote) (*pb.RemoteConfig, error) {
	remote.adjustPartialNames()

	serviceConfigs, err := mapSliceWithErrors(remote.AssociatedResourceConfigs, AssociatedResourceConfigToProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	remoteAuth, err := remoteAuthToProto(&remote.Auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert remote auth config")
	}

	proto := pb.RemoteConfig{
		Name:                    remote.Name,
		Address:                 remote.Address,
		ManagedBy:               remote.ManagedBy,
		Insecure:                remote.Insecure,
		ConnectionCheckInterval: durationpb.New(remote.ConnectionCheckInterval),
		ReconnectInterval:       durationpb.New(remote.ReconnectInterval),
		ServiceConfigs:          serviceConfigs,
		Secret:                  remote.Secret,
		Auth:                    remoteAuth,
	}

	if remote.Frame != nil {
		frame, err := FrameConfigToProto(*remote.Frame)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert frame to proto config")
		}
		proto.Frame = frame
	}

	return &proto, nil
}

// RemoteConfigFromProto creates Remote from the proto equivalent.
func RemoteConfigFromProto(proto *pb.RemoteConfig) (*Remote, error) {
	associatedResourceConfigs, err := mapSliceWithErrors(proto.ServiceConfigs, AssociatedResourceConfigFromProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	remote := Remote{
		Name:                      proto.GetName(),
		Address:                   proto.GetAddress(),
		ManagedBy:                 proto.GetManagedBy(),
		Insecure:                  proto.GetInsecure(),
		ConnectionCheckInterval:   proto.ConnectionCheckInterval.AsDuration(),
		ReconnectInterval:         proto.ReconnectInterval.AsDuration(),
		AssociatedResourceConfigs: associatedResourceConfigs,
		Secret:                    proto.GetSecret(),
	}

	if proto.GetAuth() != nil {
		remoteAuth, err := remoteAuthFromProto(proto.GetAuth())
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert remote auth config")
		}
		remote.Auth = *remoteAuth
	}

	if proto.GetFrame() != nil {
		remote.Frame, err = FrameConfigFromProto(proto.GetFrame())
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert frame from proto config")
		}
	}

	return &remote, nil
}

// NetworkConfigToProto converts NetworkConfig from the proto equivalent.
func NetworkConfigToProto(network *NetworkConfig) (*pb.NetworkConfig, error) {
	proto := pb.NetworkConfig{
		Fqdn:        network.FQDN,
		BindAddress: network.BindAddress,
		TlsCertFile: network.TLSCertFile,
		TlsKeyFile:  network.TLSKeyFile,
		Sessions:    sessionsConfigToProto(network.Sessions),
	}

	return &proto, nil
}

// NetworkConfigFromProto creates NetworkConfig from the proto equivalent.
func NetworkConfigFromProto(proto *pb.NetworkConfig) (*NetworkConfig, error) {
	network := NetworkConfig{
		NetworkConfigData: NetworkConfigData{
			FQDN:        proto.GetFqdn(),
			BindAddress: proto.GetBindAddress(),
			TLSCertFile: proto.GetTlsCertFile(),
			TLSKeyFile:  proto.GetTlsKeyFile(),
			Sessions:    sessionsConfigFromProto(proto.GetSessions()),
		},
	}

	return &network, nil
}

// AuthConfigToProto converts AuthConfig to the proto equivalent.
func AuthConfigToProto(auth *AuthConfig) (*pb.AuthConfig, error) {
	handlers, err := mapSliceWithErrors(auth.Handlers, authHandlerConfigToProto)
	if err != nil {
		return nil, err
	}

	proto := pb.AuthConfig{
		Handlers:        handlers,
		TlsAuthEntities: auth.TLSAuthEntities,
	}

	if auth.ExternalAuthConfig != nil {
		jwksJSON, err := protoutils.StructToStructPb(auth.ExternalAuthConfig.JSONKeySet)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert JSONKeySet")
		}

		proto.ExternalAuthConfig = &pb.ExternalAuthConfig{
			Jwks: &pb.JWKSFile{Json: jwksJSON},
		}
	}

	return &proto, nil
}

// AuthConfigFromProto creates AuthConfig from the proto equivalent.
func AuthConfigFromProto(proto *pb.AuthConfig) (*AuthConfig, error) {
	handlers, err := mapSliceWithErrors(proto.Handlers, authHandlerConfigFromProto)
	if err != nil {
		return nil, err
	}

	auth := AuthConfig{
		Handlers:        handlers,
		TLSAuthEntities: proto.GetTlsAuthEntities(),
	}

	if proto.ExternalAuthConfig != nil {
		auth.ExternalAuthConfig = &ExternalAuthConfig{
			JSONKeySet: proto.ExternalAuthConfig.Jwks.Json.AsMap(),
		}
	}

	return &auth, nil
}

// CloudConfigToProto converts Cloud to the proto equivalent.
func CloudConfigToProto(cloud *Cloud) (*pb.CloudConfig, error) {
	locationSecrets, err := mapSliceWithErrors(cloud.LocationSecrets, locationSecretToProto)
	if err != nil {
		return nil, err
	}

	return &pb.CloudConfig{
		Id:                cloud.ID,
		Secret:            cloud.Secret,
		LocationSecret:    cloud.LocationSecret,
		LocationSecrets:   locationSecrets,
		ManagedBy:         cloud.ManagedBy,
		Fqdn:              cloud.FQDN,
		LocalFqdn:         cloud.LocalFQDN,
		SignalingAddress:  cloud.SignalingAddress,
		SignalingInsecure: cloud.SignalingInsecure,
	}, nil
}

// CloudConfigFromProto creates Cloud from the proto equivalent.
func CloudConfigFromProto(proto *pb.CloudConfig) (*Cloud, error) {
	locationSecrets, err := mapSliceWithErrors(proto.LocationSecrets, locationSecretFromProto)
	if err != nil {
		return nil, err
	}

	return &Cloud{
		ID:     proto.GetId(),
		Secret: proto.GetSecret(),
		//nolint:staticcheck
		LocationSecret:    proto.GetLocationSecret(),
		LocationSecrets:   locationSecrets,
		ManagedBy:         proto.GetManagedBy(),
		FQDN:              proto.GetFqdn(),
		LocalFQDN:         proto.GetLocalFqdn(),
		SignalingAddress:  proto.GetSignalingAddress(),
		SignalingInsecure: proto.GetSignalingInsecure(),
	}, nil
}

func sessionsConfigToProto(sessions SessionsConfig) *pb.SessionsConfig {
	return &pb.SessionsConfig{
		HeartbeatWindow: durationpb.New(sessions.HeartbeatWindow),
	}
}

func sessionsConfigFromProto(proto *pb.SessionsConfig) SessionsConfig {
	return SessionsConfig{
		HeartbeatWindow: proto.GetHeartbeatWindow().AsDuration(),
	}
}

func locationSecretToProto(secret LocationSecret) (*pb.LocationSecret, error) {
	return &pb.LocationSecret{Id: secret.ID, Secret: secret.Secret}, nil
}

func locationSecretFromProto(secret *pb.LocationSecret) (LocationSecret, error) {
	return LocationSecret{ID: secret.Id, Secret: secret.Secret}, nil
}

func authHandlerConfigToProto(handler AuthHandlerConfig) (*pb.AuthHandlerConfig, error) {
	attributes, err := protoutils.StructToStructPb(handler.Config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert attributes configs")
	}

	credType, err := credentialsTypeToProto(handler.Type)
	if err != nil {
		return nil, err
	}

	out := &pb.AuthHandlerConfig{
		Config: attributes,
		Type:   credType,
	}

	return out, nil
}

func authHandlerConfigFromProto(proto *pb.AuthHandlerConfig) (AuthHandlerConfig, error) {
	var handler AuthHandlerConfig
	credType, err := credentialsTypeFromProto(proto.GetType())
	if err != nil {
		return handler, err
	}

	return AuthHandlerConfig{
		Config: proto.GetConfig().AsMap(),
		Type:   credType,
	}, nil
}

func credentialsTypeToProto(ct rpc.CredentialsType) (pb.CredentialsType, error) {
	switch ct {
	case rpc.CredentialsTypeAPIKey:
		return pb.CredentialsType_CREDENTIALS_TYPE_API_KEY, nil
	case rutils.CredentialsTypeRobotSecret:
		return pb.CredentialsType_CREDENTIALS_TYPE_ROBOT_SECRET, nil
	case rutils.CredentialsTypeRobotLocationSecret:
		return pb.CredentialsType_CREDENTIALS_TYPE_ROBOT_LOCATION_SECRET, nil
	case rpc.CredentialsTypeExternal:
		fallthrough
	default:
		return pb.CredentialsType_CREDENTIALS_TYPE_UNSPECIFIED, errors.New("unsupported credential type")
	}
}

func credentialsTypeFromProto(ct pb.CredentialsType) (rpc.CredentialsType, error) {
	switch ct {
	case pb.CredentialsType_CREDENTIALS_TYPE_API_KEY:
		return rpc.CredentialsTypeAPIKey, nil
	case pb.CredentialsType_CREDENTIALS_TYPE_ROBOT_SECRET:
		return rutils.CredentialsTypeRobotSecret, nil
	case pb.CredentialsType_CREDENTIALS_TYPE_ROBOT_LOCATION_SECRET:
		return rutils.CredentialsTypeRobotLocationSecret, nil
	case pb.CredentialsType_CREDENTIALS_TYPE_UNSPECIFIED:
		fallthrough
	case pb.CredentialsType_CREDENTIALS_TYPE_INTERNAL:
		fallthrough
	default:
		return "", errors.New("unsupported credential type")
	}
}

func remoteAuthToProto(auth *RemoteAuth) (*pb.RemoteAuth, error) {
	proto := pb.RemoteAuth{
		Entity: auth.Entity,
	}

	if auth.Credentials != nil {
		credType, err := credentialsTypeToProto(auth.Credentials.Type)
		if err != nil {
			return nil, err
		}

		proto.Credentials = &pb.RemoteAuth_Credentials{
			Payload: auth.Credentials.Payload,
			Type:    credType,
		}
	}

	return &proto, nil
}

func remoteAuthFromProto(proto *pb.RemoteAuth) (*RemoteAuth, error) {
	auth := RemoteAuth{
		Entity: proto.Entity,
	}

	if proto.Credentials != nil {
		credType, err := credentialsTypeFromProto(proto.Credentials.GetType())
		if err != nil {
			return nil, err
		}

		auth.Credentials = &rpc.Credentials{
			Payload: proto.GetCredentials().GetPayload(),
			Type:    credType,
		}
	}

	return &auth, nil
}

func mapSliceWithErrors[T, U any](a []T, f func(T) (U, error)) ([]U, error) {
	n := make([]U, 0, len(a))
	for _, e := range a {
		x, err := f(e)
		if err != nil {
			return nil, err
		}
		n = append(n, x)
	}
	return n, nil
}

func toRDKSlice[PT, RT any](
	protoList []*PT,
	toRDK func(*PT) (*RT, error),
	disablePartialStart bool,
	logger logging.Logger,
) ([]RT, error) {
	out := make([]RT, 0, len(protoList))
	for _, proto := range protoList {
		rdk, err := toRDK(proto)
		if err != nil {
			logger.Debugw("error converting from proto to config", "type", reflect.TypeOf(proto).String(), "error", err)
			if disablePartialStart {
				return nil, err
			}
		} else {
			out = append(out, *rdk)
		}
	}
	return out, nil
}

// PackageConfigToProto converts a rdk package config to the proto version.
func PackageConfigToProto(cfg *PackageConfig) (*pb.PackageConfig, error) {
	var status *pb.AppValidationStatus
	if cfg.Status != nil {
		status = &pb.AppValidationStatus{Error: cfg.Status.Error}
	}

	return &pb.PackageConfig{
		Name:    cfg.Name,
		Package: cfg.Package,
		Version: cfg.Version,
		Type:    string(cfg.Type),
		Status:  status,
	}, nil
}

// PackageConfigFromProto converts a proto package config to the rdk version.
func PackageConfigFromProto(proto *pb.PackageConfig) (*PackageConfig, error) {
	var status *AppValidationStatus
	if proto.GetStatus() != nil {
		status = &AppValidationStatus{Error: proto.GetStatus().GetError()}
	}

	return &PackageConfig{
		Name:    proto.Name,
		Package: proto.Package,
		Version: proto.Version,
		Type:    PackageType(proto.Type),
		Status:  status,
	}, nil
}

// PackageTypeToProto converts a config PackageType to its proto equivalent
// This is required be because app/packages uses a PackageType enum but app/PackageConfig uses a string Type.
func PackageTypeToProto(t PackageType) (*packagespb.PackageType, error) {
	switch t {
	case PackageTypeMlModel:
		return packagespb.PackageType_PACKAGE_TYPE_ML_MODEL.Enum(), nil
	case PackageTypeModule:
		return packagespb.PackageType_PACKAGE_TYPE_MODULE.Enum(), nil
	case PackageTypeSlamMap:
		return packagespb.PackageType_PACKAGE_TYPE_SLAM_MAP.Enum(), nil
	case PackageTypeBoardDefs:
		return packagespb.PackageType_PACKAGE_TYPE_BOARD_DEFS.Enum(), nil
	default:
		return packagespb.PackageType_PACKAGE_TYPE_UNSPECIFIED.Enum(), errors.Errorf("unknown package type %q", t)
	}
}
