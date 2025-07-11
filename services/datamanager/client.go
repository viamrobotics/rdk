// Package datamanager contains a gRPC based datamanager service server
package datamanager

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"image/png"

	datasyncpb "go.viam.com/api/app/datasync/v1"
	pb "go.viam.com/api/service/datamanager/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements DataManagerServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.DataManagerServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Service, error) {
	grpcClient := pb.NewDataManagerServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: grpcClient,
		logger: logger,
	}
	return c, nil
}

func (c *client) Sync(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Sync(ctx, &pb.SyncRequest{Name: c.name, Extra: ext})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) UploadRawDataToDataset(ctx context.Context, image []byte, datasetIDs, tags []string, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.UploadImageToDataset(ctx, &pb.UploadImageToDatasetRequest{
		Image:      image,
		DatasetIds: datasetIDs,
		Tags:       tags,
		Extra:      ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) UploadImageToDataset(
	ctx context.Context,
	image image.Image,
	datasetIDs, tags []string,
	mimeType datasyncpb.MimeType,
	extra map[string]interface{},
) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}

	var imgBytes []byte
	var buf bytes.Buffer
	switch mimeType {
	case datasyncpb.MimeType_MIME_TYPE_IMAGE_JPEG:
		err := jpeg.Encode(&buf, image, nil)
		if err != nil {
			return err
		}
		imgBytes = buf.Bytes()
	case datasyncpb.MimeType_MIME_TYPE_IMAGE_PNG:
		err := png.Encode(&buf, image)
		if err != nil {
			return err
		}
		imgBytes = buf.Bytes()
	default:
		return errors.New("mime type must png or jpeg")
	}
	_, err = c.client.UploadImageToDataset(ctx, &pb.UploadImageToDatasetRequest{
		Image:      imgBytes,
		DatasetIds: datasetIDs,
		Tags:       tags,
		Extra:      ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
