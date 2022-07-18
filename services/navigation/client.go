package navigation

import (
	"context"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/utils/rpc"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/navigation/v1"
)

// client is a client satisfies the frame_system.proto contract.
type client struct {
	conn   rpc.ClientConn
	client pb.NavigationServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewNavigationServiceClient(conn)
	sc := &client{
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return sc
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	return newSvcClientFromConn(conn, logger)
}

func (c *client) GetMode(ctx context.Context) (Mode, error) {
	resp, err := c.client.GetMode(ctx, &pb.GetModeRequest{})
	if err != nil {
		return 0, err
	}
	pbMode := resp.GetMode()
	switch pbMode {
	case pb.Mode_MODE_MANUAL:
		return ModeManual, nil
	case pb.Mode_MODE_WAYPOINT:
		return ModeWaypoint, nil
	case pb.Mode_MODE_UNSPECIFIED:
		fallthrough
	default:
		return 0, errors.New("mode error")
	}
}

func (c *client) SetMode(ctx context.Context, mode Mode) error {
	var pbMode pb.Mode
	switch mode {
	case ModeManual:
		pbMode = pb.Mode_MODE_MANUAL
	case ModeWaypoint:
		pbMode = pb.Mode_MODE_WAYPOINT
	default:
		pbMode = pb.Mode_MODE_UNSPECIFIED
	}
	_, err := c.client.SetMode(ctx, &pb.SetModeRequest{Mode: pbMode})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) GetLocation(ctx context.Context) (*geo.Point, error) {
	resp, err := c.client.GetLocation(ctx, &pb.GetLocationRequest{})
	if err != nil {
		return nil, err
	}
	loc := resp.GetLocation()
	result := geo.NewPoint(loc.GetLatitude(), loc.GetLongitude())
	return result, nil
}

func (c *client) GetWaypoints(ctx context.Context) ([]Waypoint, error) {
	resp, err := c.client.GetWaypoints(ctx, &pb.GetWaypointsRequest{})
	if err != nil {
		return nil, err
	}
	waypoints := resp.GetWaypoints()
	result := make([]Waypoint, 0, len(waypoints))
	for _, wpt := range waypoints {
		id, err := primitive.ObjectIDFromHex(wpt.GetId())
		if err != nil {
			return nil, err
		}
		loc := wpt.GetLocation()
		result = append(result, Waypoint{
			ID:   id,
			Lat:  loc.GetLatitude(),
			Long: loc.GetLongitude(),
		})
	}
	return result, nil
}

func (c *client) AddWaypoint(ctx context.Context, point *geo.Point) error {
	loc := &commonpb.GeoPoint{
		Latitude:  point.Lat(),
		Longitude: point.Lng(),
	}
	req := &pb.AddWaypointRequest{
		Location: loc,
	}
	_, err := c.client.AddWaypoint(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error {
	req := &pb.RemoveWaypointRequest{Id: id.Hex()}
	_, err := c.client.RemoveWaypoint(ctx, req)
	if err != nil {
		return err
	}
	return nil
}
