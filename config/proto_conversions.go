package config

import (
	"github.com/pkg/errors"
	pb "go.viam.com/api/proto/viam/app/v1"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	spatial "go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

// ComponentConfigToProto converts Component to proto equivalent.
func ComponentConfigToProto(component *Component) (*pb.ComponentConfig, error) {
	attributes, err := protoutils.StructToStructPb(component.Attributes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert attributes configs")
	}

	serviceConfigs, err := mapSliceWithErrors(component.ServiceConfig, ResourceLevelServiceConfigToProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	proto := pb.ComponentConfig{
		Name:           component.Name,
		Namespace:      string(component.Namespace),
		Type:           string(component.Type),
		SubType:        component.SubType,
		Model:          component.Model,
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

// ComponentConfigFromProto creates Component from proto equivalent.
func ComponentConfigFromProto(proto *pb.ComponentConfig) (*Component, error) {
	serviceConfigs, err := mapSliceWithErrors(proto.ServiceConfigs, ResourceLevelServiceConfigFromProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	component := Component{
		Name:          proto.GetName(),
		Type:          resource.SubtypeName(proto.GetType()),
		Namespace:     resource.Namespace(proto.GetNamespace()),
		SubType:       proto.GetSubType(),
		Model:         proto.GetModel(),
		Attributes:    proto.GetAttributes().AsMap(),
		DependsOn:     proto.GetDependsOn(),
		ServiceConfig: serviceConfigs,
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

// ServiceConfigToProto converts Service to proto equivalent.
func ServiceConfigToProto(service *Service) (*pb.ServiceConfig, error) {
	attributes, err := protoutils.StructToStructPb(service.Attributes)
	if err != nil {
		return nil, err
	}

	proto := pb.ServiceConfig{
		Name:       service.Name,
		Namespace:  string(service.Namespace),
		Type:       string(service.Type),
		Attributes: attributes,
	}

	return &proto, nil
}

// ServiceConfigFromProto creates Service from proto equivalent.
func ServiceConfigFromProto(proto *pb.ServiceConfig) (*Service, error) {
	service := Service{
		Name:       proto.GetName(),
		Namespace:  resource.Namespace(proto.GetNamespace()),
		Type:       ServiceType(proto.GetType()),
		Attributes: proto.GetAttributes().AsMap(),
	}

	return &service, nil
}

// ProcessConfigToProto converts ProcessConfig to proto equivalent.
func ProcessConfigToProto(process *pexec.ProcessConfig) (*pb.ProcessConfig, error) {
	return &pb.ProcessConfig{
		Id:      process.ID,
		Name:    process.Name,
		Args:    process.Args,
		Cwd:     process.CWD,
		OneShot: process.OneShot,
		Log:     process.Log,
	}, nil
}

// ProcessConfigFromProto creates ProcessConfig from proto equivalent.
func ProcessConfigFromProto(proto *pb.ProcessConfig) (*pexec.ProcessConfig, error) {
	return &pexec.ProcessConfig{
		ID:      proto.Id,
		Name:    proto.Name,
		Args:    proto.Args,
		CWD:     proto.Cwd,
		OneShot: proto.OneShot,
		Log:     proto.Log,
	}, nil
}

// ResourceLevelServiceConfigToProto converts ResourceLevelServiceConfig to proto equivalent.
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

// ResourceLevelServiceConfigFromProto creates ResourceLevelServiceConfig from proto equivalent.
func ResourceLevelServiceConfigFromProto(proto *pb.ResourceLevelServiceConfig) (ResourceLevelServiceConfig, error) {
	service := ResourceLevelServiceConfig{
		Type:       resource.SubtypeName(proto.GetType()),
		Attributes: proto.GetAttributes().AsMap(),
	}

	return service, nil
}

// FrameConfigToProto converts Frame to proto equivalent.
func FrameConfigToProto(frame Frame) (*pb.Frame, error) {
	proto := pb.Frame{
		Parent: frame.Parent,
		Translation: &pb.Translation{
			X: frame.Translation.X,
			Y: frame.Translation.Y,
			Z: frame.Translation.Z,
		},
	}

	var orientation pb.Orientation

	switch oType := frame.Orientation.(type) {
	case *spatial.R4AA:
		r4aa := frame.Orientation.(*spatial.R4AA)
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
		vector := frame.Orientation.(*spatial.OrientationVector)
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
		vector := frame.Orientation.(*spatial.OrientationVectorDegrees)
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
		eulerAngles := frame.Orientation.(*spatial.EulerAngles)
		config := pb.Orientation_EulerAngles{
			Roll:  eulerAngles.Roll,
			Pitch: eulerAngles.Pitch,
			Yaw:   eulerAngles.Yaw,
		}
		orientation = pb.Orientation{
			Type: &pb.Orientation_EulerAngles_{EulerAngles: &config},
		}
	case *spatial.Quaternion:
		q := frame.Orientation.(*spatial.Quaternion)
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

	return &proto, nil
}

// FrameConfigFromProto creates Frame from proto equivalent.
func FrameConfigFromProto(proto *pb.Frame) (*Frame, error) {
	frame := Frame{
		Parent: proto.GetParent(),
		Translation: spatial.TranslationConfig{
			X: proto.GetTranslation().GetX(),
			Y: proto.GetTranslation().GetY(),
			Z: proto.GetTranslation().GetZ(),
		},
	}

	if proto.GetOrientation() != nil {
		switch or := proto.GetOrientation().Type.(type) {
		case *pb.Orientation_NoOrientation_:
			frame.Orientation = spatial.NewZeroOrientation()
		case *pb.Orientation_VectorRadians:
			frame.Orientation = &spatial.OrientationVector{
				Theta: or.VectorRadians.Theta,
				OX:    or.VectorRadians.X,
				OY:    or.VectorRadians.Y,
				OZ:    or.VectorRadians.Z,
			}
		case *pb.Orientation_VectorDegrees:
			frame.Orientation = &spatial.OrientationVectorDegrees{
				Theta: or.VectorDegrees.Theta,
				OX:    or.VectorDegrees.X,
				OY:    or.VectorDegrees.Y,
				OZ:    or.VectorDegrees.Z,
			}
		case *pb.Orientation_EulerAngles_:
			frame.Orientation = &spatial.EulerAngles{
				Pitch: or.EulerAngles.Pitch,
				Roll:  or.EulerAngles.Roll,
				Yaw:   or.EulerAngles.Yaw,
			}
		case *pb.Orientation_AxisAngles_:
			frame.Orientation = &spatial.R4AA{
				Theta: or.AxisAngles.Theta,
				RX:    or.AxisAngles.X,
				RY:    or.AxisAngles.Y,
				RZ:    or.AxisAngles.Z,
			}
		case *pb.Orientation_Quaternion_:
			frame.Orientation = &spatial.Quaternion{
				Real: or.Quaternion.W,
				Imag: or.Quaternion.X,
				Jmag: or.Quaternion.Y,
				Kmag: or.Quaternion.Z,
			}
		default:
			return nil, errors.New("Orientation type unsupported")
		}
	}

	return &frame, nil
}

// RemoteConfigToProto converts Remote to proto equivalent.
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
		Prefix:                  remote.Prefix,
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

// RemoteConfigFromProto creates Remote from proto equivalent.
func RemoteConfigFromProto(proto *pb.RemoteConfig) (*Remote, error) {
	serviceConfigs, err := mapSliceWithErrors(proto.ServiceConfigs, ResourceLevelServiceConfigFromProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert service configs")
	}

	remote := Remote{
		Name:                    proto.GetName(),
		Address:                 proto.GetAddress(),
		Prefix:                  proto.GetPrefix(),
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

// NetworkConfigToProto converts NetworkConfig from proto equivalent.
func NetworkConfigToProto(network *NetworkConfig) (*pb.NetworkConfig, error) {
	proto := pb.NetworkConfig{
		Fqdn:        network.FQDN,
		BindAddress: network.BindAddress,
		TlsCertFile: network.TLSCertFile,
		TlsKeyFile:  network.TLSKeyFile,
	}

	return &proto, nil
}

// NetworkConfigFromProto creates NetworkConfig from proto equivalent.
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

// AuthConfigToProto converts AuthConfig to proto equivalent.
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

// AuthConfigFromProto creates AuthConfig from proto equivalent.
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

// CloudConfigToProto converts Cloud to proto equivalent.
func CloudConfigToProto(cloud *Cloud) (*pb.CloudConfig, error) {
	return &pb.CloudConfig{
		Id:                cloud.ID,
		Secret:            cloud.Secret,
		LocationSecret:    cloud.LocationSecret,
		ManagedBy:         cloud.ManagedBy,
		Fqdn:              cloud.FQDN,
		LocalFqdn:         cloud.LocalFQDN,
		SignalingAddress:  cloud.SignalingAddress,
		SignalingInsecure: cloud.SignalingInsecure,
	}, nil
}

// CloudConfigFromProto creates Cloud from proto equivalent.
func CloudConfigFromProto(proto *pb.CloudConfig) (*Cloud, error) {
	return &Cloud{
		ID:                proto.GetId(),
		Secret:            proto.GetSecret(),
		LocationSecret:    proto.GetLocationSecret(),
		ManagedBy:         proto.GetManagedBy(),
		FQDN:              proto.GetFqdn(),
		LocalFQDN:         proto.GetLocalFqdn(),
		SignalingAddress:  proto.GetSignalingAddress(),
		SignalingInsecure: proto.GetSignalingInsecure(),
	}, nil
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

	return &pb.AuthHandlerConfig{
		Config: attributes,
		Type:   credType,
	}, nil
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
