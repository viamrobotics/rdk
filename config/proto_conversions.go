package config

import (
	"reflect"
	"strings"
	"syscall"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/rpc/oauth"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	spatial "go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

// FromProto converts the RobotConfig to the internal rdk equivalent.
func FromProto(proto *pb.RobotConfig) (*Config, error) {
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
	cfg.Modules, err = toRDKSlice(proto.Modules, ModuleConfigFromProto, disablePartialStart)
	if err != nil {
		return nil, errors.Wrap(err, "error converting modules config from proto")
	}

	cfg.Components, err = toRDKSlice(proto.Components, ComponentConfigFromProto, disablePartialStart)
	if err != nil {
		return nil, errors.Wrap(err, "error converting components config from proto")
	}

	cfg.Remotes, err = toRDKSlice(proto.Remotes, RemoteConfigFromProto, disablePartialStart)
	if err != nil {
		return nil, errors.Wrap(err, "error converting remotes config from proto")
	}

	cfg.Processes, err = toRDKSlice(proto.Processes, ProcessConfigFromProto, disablePartialStart)
	if err != nil {
		return nil, errors.Wrap(err, "error converting processes config from proto")
	}

	cfg.Services, err = toRDKSlice(proto.Services, ServiceConfigFromProto, disablePartialStart)
	if err != nil {
		return nil, errors.Wrap(err, "error converting services config from proto")
	}

	if proto.Debug != nil {
		cfg.Debug = *proto.Debug
	}

	return &cfg, nil
}

// ComponentConfigToProto converts Component to the proto equivalent.
func ComponentConfigToProto(component *Component) (*pb.ComponentConfig, error) {
	attributes, err := protoutils.StructToStructPb(component.Attributes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert attributes configs")
	}

	serviceConfigs, err := mapSliceWithErrors(component.ServiceConfig, ResourceLevelServiceConfigToProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	if err := component.fixAPI(); err != nil {
		return nil, errors.Wrap(err, "failed to convert namespace/type/api config")
	}

	proto := pb.ComponentConfig{
		Name:           component.Name,
		Namespace:      string(component.Namespace),
		Type:           string(component.Type),
		Api:            component.API.String(),
		Model:          component.Model.String(),
		DependsOn:      component.DependsOn,
		ServiceConfigs: serviceConfigs,
		Attributes:     attributes,
	}

	if component.Frame != nil {
		frame, err := FrameConfigToProto(*component.Frame)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert frame to proto config")
		}
		proto.Frame = frame
	}

	return &proto, nil
}

// ComponentConfigFromProto creates Component from the proto equivalent.
func ComponentConfigFromProto(proto *pb.ComponentConfig) (*Component, error) {
	serviceConfigs, err := mapSliceWithErrors(proto.ServiceConfigs, ResourceLevelServiceConfigFromProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	// for consistency, nil out empty maps and configs (otherwise go>proto>go conversion doesn't match)
	attrs := proto.GetAttributes().AsMap()
	if len(attrs) == 0 {
		attrs = nil
	}

	if len(serviceConfigs) == 0 {
		serviceConfigs = nil
	}

	component := Component{
		Name:          proto.GetName(),
		Type:          resource.SubtypeName(proto.GetType()),
		Namespace:     resource.Namespace(proto.GetNamespace()),
		Model:         resource.NewModelFromStringIgnoreErrors(proto.GetModel()),
		Attributes:    attrs,
		DependsOn:     proto.GetDependsOn(),
		ServiceConfig: serviceConfigs,
	}

	if strings.ContainsRune(proto.GetApi(), ':') {
		component.API, err = resource.NewSubtypeFromString(proto.GetApi())
		if err != nil {
			return nil, err
		}
	}

	if err := component.fixAPI(); err != nil {
		return nil, err
	}

	if proto.GetFrame() != nil {
		frame, err := FrameConfigFromProto(proto.GetFrame())
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert frame from proto config")
		}
		component.Frame = frame
	}

	return &component, nil
}

// ServiceConfigToProto converts Service to the proto equivalent.
func ServiceConfigToProto(service *Service) (*pb.ServiceConfig, error) {
	attributes, err := protoutils.StructToStructPb(service.Attributes)
	if err != nil {
		return nil, err
	}

	proto := pb.ServiceConfig{
		Name:       service.Name,
		Namespace:  string(service.Namespace),
		Type:       string(service.Type),
		Model:      service.Model.String(),
		Attributes: attributes,
		DependsOn:  service.DependsOn,
	}

	return &proto, nil
}

// ServiceConfigToSharedProto converts Service to the proto equivalent shared with Components.
func ServiceConfigToSharedProto(service *Service) (*pb.ComponentConfig, error) {
	attributes, err := protoutils.StructToStructPb(service.Attributes)
	if err != nil {
		return nil, err
	}

	proto := pb.ComponentConfig{
		Name:       service.Name,
		Namespace:  string(service.Namespace),
		Type:       string(service.Type),
		Api:        string(service.Namespace) + ":" + string(resource.ResourceTypeService) + ":" + string(service.Type),
		Model:      service.Model.String(),
		Attributes: attributes,
		DependsOn:  service.DependsOn,
	}

	return &proto, nil
}

// ServiceConfigFromProto creates Service from the proto equivalent shared with Components.
func ServiceConfigFromProto(proto *pb.ServiceConfig) (*Service, error) {
	// for consistency, nil out empty map (otherwise go>proto>go conversion doesn't match)
	attrs := proto.GetAttributes().AsMap()
	if len(attrs) == 0 {
		attrs = nil
	}

	service := Service{
		Name:       proto.GetName(),
		Namespace:  resource.Namespace(proto.GetNamespace()),
		Type:       resource.SubtypeName(proto.GetType()),
		Model:      resource.NewModelFromStringIgnoreErrors(proto.GetModel()),
		Attributes: attrs,
		DependsOn:  proto.GetDependsOn(),
	}

	return &service, nil
}

// ServiceConfigFromSharedProto creates a Service from the proto equivalent.
func ServiceConfigFromSharedProto(proto *pb.ComponentConfig) (*Service, error) {
	service := Service{
		Name:       proto.GetName(),
		Namespace:  resource.Namespace(proto.GetNamespace()),
		Type:       resource.SubtypeName(proto.GetType()),
		Model:      resource.NewModelFromStringIgnoreErrors(proto.GetModel()),
		Attributes: proto.GetAttributes().AsMap(),
		DependsOn:  proto.GetDependsOn(),
	}

	return &service, nil
}

// ModuleConfigToProto converts Module to the proto equivalent.
func ModuleConfigToProto(module *Module) (*pb.ModuleConfig, error) {
	proto := pb.ModuleConfig{
		Name: module.Name,
		Path: module.ExePath,
	}

	return &proto, nil
}

// ModuleConfigFromProto creates Module from the proto equivalent.
func ModuleConfigFromProto(proto *pb.ModuleConfig) (*Module, error) {
	module := Module{
		Name:    proto.GetName(),
		ExePath: proto.GetPath(),
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
		OneShot:     proto.OneShot,
		Log:         proto.Log,
		StopSignal:  syscall.Signal(proto.StopSignal),
		StopTimeout: proto.StopTimeout.AsDuration(),
	}, nil
}

// ResourceLevelServiceConfigToProto converts ResourceLevelServiceConfig to the proto equivalent.
func ResourceLevelServiceConfigToProto(service ResourceLevelServiceConfig) (*pb.ResourceLevelServiceConfig, error) {
	attributes, err := protoutils.StructToStructPb(service.Attributes)
	if err != nil {
		return nil, err
	}

	proto := pb.ResourceLevelServiceConfig{
		Type:       string(service.Type),
		Attributes: attributes,
	}

	return &proto, nil
}

// ResourceLevelServiceConfigFromProto creates ResourceLevelServiceConfig from the proto equivalent.
func ResourceLevelServiceConfigFromProto(proto *pb.ResourceLevelServiceConfig) (ResourceLevelServiceConfig, error) {
	service := ResourceLevelServiceConfig{
		Type:       resource.SubtypeName(proto.GetType()),
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
		geom, err := spatial.NewGeometryCreatorFromProto(proto.GetGeometry())
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
	serviceConfigs, err := mapSliceWithErrors(remote.ServiceConfig, ResourceLevelServiceConfigToProto)
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
	serviceConfigs, err := mapSliceWithErrors(proto.ServiceConfigs, ResourceLevelServiceConfigFromProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	remote := Remote{
		Name:                    proto.GetName(),
		Address:                 proto.GetAddress(),
		ManagedBy:               proto.GetManagedBy(),
		Insecure:                proto.GetInsecure(),
		ConnectionCheckInterval: proto.ConnectionCheckInterval.AsDuration(),
		ReconnectInterval:       proto.ReconnectInterval.AsDuration(),
		ServiceConfig:           serviceConfigs,
		Secret:                  proto.GetSecret(),
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

	if credType == pb.CredentialsType_CREDENTIALS_TYPE_WEB_OAUTH {
		if handler.WebOAuthConfig == nil {
			return nil, errors.New("missing WebOAuthConfig field for CREDENTIALS_TYPE_WEB_OAUTH AuthHandler type")
		}

		jwksJSON, err := protoutils.StructToStructPb(handler.WebOAuthConfig.JSONKeySet)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert JSONKeySet")
		}

		out.WebOauthConfig = &pb.AuthHandlerWebOauthConfig{
			AllowedAudiences: handler.WebOAuthConfig.AllowedAudiences,
			Jwks:             &pb.JWKSFile{Json: jwksJSON},
		}
	}

	return out, nil
}

func authHandlerConfigFromProto(proto *pb.AuthHandlerConfig) (AuthHandlerConfig, error) {
	var handler AuthHandlerConfig
	credType, err := credentialsTypeFromProto(proto.GetType())
	if err != nil {
		return handler, err
	}

	handler = AuthHandlerConfig{
		Config: proto.GetConfig().AsMap(),
		Type:   credType,
	}

	if credType == oauth.CredentialsTypeOAuthWeb {
		if proto.WebOauthConfig == nil {
			return handler, errors.New("missing WebOAuthConfig field for CredentialsTypeOAuthWeb AuthHandler type")
		}

		handler.WebOAuthConfig = &WebOAuthConfig{
			AllowedAudiences: proto.WebOauthConfig.AllowedAudiences,
			JSONKeySet:       proto.WebOauthConfig.Jwks.Json.AsMap(),
		}
	}

	return handler, nil
}

func credentialsTypeToProto(ct rpc.CredentialsType) (pb.CredentialsType, error) {
	switch ct {
	case rpc.CredentialsTypeAPIKey:
		return pb.CredentialsType_CREDENTIALS_TYPE_API_KEY, nil
	case rutils.CredentialsTypeRobotSecret:
		return pb.CredentialsType_CREDENTIALS_TYPE_ROBOT_SECRET, nil
	case rutils.CredentialsTypeRobotLocationSecret:
		return pb.CredentialsType_CREDENTIALS_TYPE_ROBOT_LOCATION_SECRET, nil
	case oauth.CredentialsTypeOAuthWeb:
		return pb.CredentialsType_CREDENTIALS_TYPE_WEB_OAUTH, nil
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
	case pb.CredentialsType_CREDENTIALS_TYPE_WEB_OAUTH:
		return oauth.CredentialsTypeOAuthWeb, nil
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

func toRDKSlice[PT, RT any](protoList []*PT, toRDK func(*PT) (*RT, error), disablePartialStart bool) ([]RT, error) {
	out := make([]RT, 0, len(protoList))
	for _, proto := range protoList {
		rdk, err := toRDK(proto)
		if err != nil {
			golog.Global().Debug(errors.Wrap(err, "Error converting from proto to config for type: "+reflect.TypeOf(proto).String()))
			if disablePartialStart {
				return nil, err
			}
		} else {
			out = append(out, *rdk)
		}
	}
	return out, nil
}

// ServiceConfigToShared converts a Service to the common resource config (Component for now.)
func ServiceConfigToShared(cfg Service) Component {
	return Component{
		Name:                cfg.Name,
		Namespace:           cfg.Namespace,
		Type:                cfg.Type,
		API:                 resource.NewSubtype(cfg.Namespace, resource.ResourceTypeService, cfg.Type),
		Model:               cfg.Model,
		DependsOn:           cfg.DependsOn,
		Attributes:          cfg.Attributes,
		ConvertedAttributes: cfg.ConvertedAttributes,
		ImplicitDependsOn:   cfg.ImplicitDependsOn,
	}
}

// ServiceConfigFromShared converts a common resource config (Component for now) to a Service.
func ServiceConfigFromShared(cfg Component) Service {
	return Service{
		Name:                cfg.Name,
		Namespace:           cfg.Namespace,
		Type:                cfg.Type,
		Model:               cfg.Model,
		DependsOn:           cfg.DependsOn,
		Attributes:          cfg.Attributes,
		ConvertedAttributes: cfg.ConvertedAttributes,
		ImplicitDependsOn:   cfg.ImplicitDependsOn,
	}
}
