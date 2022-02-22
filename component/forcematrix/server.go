// Package forcematrix contains a gRPC based ForceMatrix service server.
package forcematrix

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/component/forcematrix/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from force_matrix.proto.
type subtypeServer struct {
	pb.UnimplementedForceMatrixServiceServer
	s subtype.Service
}

// NewServer constructs a ForceMatrix gRPC service server.
func NewServer(s subtype.Service) pb.ForceMatrixServiceServer {
	return &subtypeServer{s: s}
}

// getForceMatrix returns the ForceMatrix specified, nil if not.
func (s *subtypeServer) getForceMatrix(name string) (ForceMatrix, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no ForceMatrix with name (%s)", name)
	}
	forceMatrix, ok := resource.(ForceMatrix)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a ForceMatrix", name)
	}
	return forceMatrix, nil
}

// ReadMatrix returns a matrix of measured forces on a matrix force sensor.
func (s *subtypeServer) ReadMatrix(
	ctx context.Context,
	req *pb.ReadMatrixRequest,
) (*pb.ReadMatrixResponse, error) {
	forceMatrixDevice, err := s.getForceMatrix(req.Name)
	if err != nil {
		return nil, err
	}
	matrix, err := forceMatrixDevice.ReadMatrix(ctx)
	if err != nil {
		return nil, err
	}
	return matrixToProto(matrix), nil
}

// DetectSlip returns a boolean representing whether a slip has been detected.
func (s *subtypeServer) DetectSlip(ctx context.Context,
	req *pb.DetectSlipRequest,
) (*pb.DetectSlipResponse, error) {
	forceMatrixDevice, err := s.getForceMatrix(req.Name)
	if err != nil {
		return nil, err
	}
	isSlipping, err := forceMatrixDevice.DetectSlip(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.DetectSlipResponse{SlipDetected: isSlipping}, nil
}

// matrixToProto is a helper function to convert force matrix values from a 2-dimensional
// slice into protobuf format.
func matrixToProto(matrix [][]int) *pb.ReadMatrixResponse {
	rows := len(matrix)
	var cols int
	if rows != 0 {
		cols = len(matrix[0])
	}

	data := make([]uint32, 0, rows*cols)
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			data = append(data, uint32(matrix[row][col]))
		}
	}

	return &pb.ReadMatrixResponse{Matrix: &pb.Matrix{
		Rows: uint32(rows),
		Cols: uint32(cols),
		Data: data,
	}}
}
