package navigation

import (
	"context"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/navigation/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
)

// client implements NavigationServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.NavigationServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	grpcClient := pb.NewNavigationServiceClient(conn)
	c := &client{
		name:   name,
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return c
}

func (c *client) Mode(ctx context.Context, extra map[string]interface{}) (Mode, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return 0, err
	}
	resp, err := c.client.GetMode(ctx, &pb.GetModeRequest{Name: c.name, Extra: ext})
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

func (c *client) SetMode(ctx context.Context, mode Mode, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	var pbMode pb.Mode
	switch mode {
	case ModeManual:
		pbMode = pb.Mode_MODE_MANUAL
	case ModeWaypoint:
		pbMode = pb.Mode_MODE_WAYPOINT
	default:
		pbMode = pb.Mode_MODE_UNSPECIFIED
	}
	_, err = c.client.SetMode(ctx, &pb.SetModeRequest{Name: c.name, Mode: pbMode, Extra: ext})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Location(ctx context.Context, extra map[string]interface{}) (*geo.Point, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetLocation(ctx, &pb.GetLocationRequest{Name: c.name, Extra: ext})
	if err != nil {
		return nil, err
	}
	loc := resp.GetLocation()
	result := geo.NewPoint(loc.GetLatitude(), loc.GetLongitude())
	return result, nil
}

func (c *client) Waypoints(ctx context.Context, extra map[string]interface{}) ([]Waypoint, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetWaypoints(ctx, &pb.GetWaypointsRequest{Name: c.name, Extra: ext})
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

func (c *client) AddWaypoint(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	loc := &commonpb.GeoPoint{
		Latitude:  point.Lat(),
		Longitude: point.Lng(),
	}
	req := &pb.AddWaypointRequest{
		Name:     c.name,
		Location: loc,
		Extra:    ext,
	}
	_, err = c.client.AddWaypoint(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	req := &pb.RemoveWaypointRequest{Name: c.name, Id: id.Hex(), Extra: ext}
	_, err = c.client.RemoveWaypoint(ctx, req)
	if err != nil {
		return err
	}
	return nil
}
