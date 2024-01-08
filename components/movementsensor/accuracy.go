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
	var hdop, vdop, compassError float32
	var nmeaFix int32

	if resp.PositionHdop != nil {
		hdop = *resp.PositionHdop
	}
	if resp.PositionVdop != nil {
		vdop = *resp.PositionVdop
	}
	if resp.PositionNmeaGgaFix != nil {
		nmeaFix = *resp.PositionNmeaGgaFix
	}
	if resp.CompassDegreesError != nil {
		compassError = *resp.CompassDegreesError
	}

	return &Accuracy{
		AccuracyMap:        resp.Accuracy,
		Hdop:               hdop,
		Vdop:               vdop,
		NmeaFix:            nmeaFix,
		CompassDegreeError: compassError,
	}
}

func AccuracyToProtoResponse(acc *Accuracy) (*pb.GetAccuracyResponse, error) {
	return &pb.GetAccuracyResponse{
		Accuracy:            acc.AccuracyMap,
		PositionHdop:        &acc.Hdop,
		PositionVdop:        &acc.Vdop,
		PositionNmeaGgaFix:  &acc.NmeaFix,
		CompassDegreesError: &acc.CompassDegreeError,
	}, nil
}
