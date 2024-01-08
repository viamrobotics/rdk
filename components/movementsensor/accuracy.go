package movementsensor

import pb "go.viam.com/api/component/movementsensor/v1"

type Accuracy struct {
	AccuracyMap        map[string]float32
	Hdop               float32
	Vdop               float32
	NmeaFix            int32
	CompassDegreeError float32
}

func ProtoFeaturesToAccuracy(resp *pb.GetAccuracyResponse) *Accuracy {
	return &Accuracy{
		AccuracyMap:        resp.Accuracy,
		Hdop:               *resp.PositionHdop,
		Vdop:               *resp.PositionVdop,
		NmeaFix:            *resp.PositionNmeaGgaFix,
		CompassDegreeError: *resp.CompassDegreesError,
	}
}

func AccuracyToProtoResponse(
	features *Accuracy,
) (*pb.GetAccuracyResponse, error) {
	return &pb.GetAccuracyResponse{
		Accuracy:            features.AccuracyMap,
		PositionHdop:        &features.Hdop,
		PositionVdop:        &features.Vdop,
		PositionNmeaGgaFix:  &features.NmeaFix,
		CompassDegreesError: &features.CompassDegreeError,
	}, nil
}
