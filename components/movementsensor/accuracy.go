package movementsensor

import (
	pb "go.viam.com/api/component/movementsensor/v1"
)

// Accuracy defines the precision and reliability metrics of a movement sensor.
// It includes various parameters to assess the sensor's accuracy in different dimensions.
//
// Fields:
//
// AccuracyMap: A mapping of specific measurement parameters to their accuracy values.
// The keys are string identifiers for each measurement (e.g., "Hdop", "Vdop"),
// and the values are their corresponding accuracy levels as float32.
//
// Hdop: Horizontal Dilution of Precision (HDOP) value. It indicates the level of accuracy
// of horizontal measurements. Lower values represent higher precision.
//
// Vdop: Vertical Dilution of Precision (VDOP) value. Similar to HDOP, it denotes the
// accuracy level of vertical measurements. Lower VDOP values signify better precision.
//
// NmeaFix: An integer value representing the quality of the NMEA fix.
// Higher values generally indicate a better quality fix, with specific meanings depending
// on the sensor and context. Generally a fix of 1 or 2 lends to large position errors,
// ideally we want a fix of 4-5. Other fixes are unsuitable for outdoor navigation.
// The meaning of each fix value is documented here
// https://docs.novatel.com/OEM7/Content/Logs/GPGGA.htm#GPSQualityIndicators
//
// CompassDegreeError: The estimated error in compass readings, measured in degrees.
// It signifies the deviation or uncertainty in the sensor's compass measurements.
// A lower value implies a more accurate compass direction.
type Accuracy struct {
	AccuracyMap        map[string]float32
	Hdop               float32
	Vdop               float32
	NmeaFix            int32
	CompassDegreeError float32
}

// ProtoFeaturesToAccuracy converts a GetAccuracyResponse from a protocol buffer (protobuf)
// into an Accuracy struct.
func protoFeaturesToAccuracy(resp *pb.GetAccuracyResponse) *Accuracy {
	if resp == nil {
		return UnimplementedOptionalAccuracies()
	}

	return &Accuracy{
		AccuracyMap:        resp.Accuracy,
		Hdop:               *resp.PositionHdop,
		Vdop:               *resp.PositionVdop,
		NmeaFix:            *resp.PositionNmeaGgaFix,
		CompassDegreeError: *resp.CompassDegreesError,
	}
}

// AccuracyToProtoResponse converts an Accuracy struct into a protobuf GetAccuracyResponse.
func accuracyToProtoResponse(acc *Accuracy) (*pb.GetAccuracyResponse, error) {
	if acc == nil {
		uacc := UnimplementedOptionalAccuracies()
		return &pb.GetAccuracyResponse{
			Accuracy:            map[string]float32{},
			PositionHdop:        &uacc.Hdop,
			PositionVdop:        &uacc.Vdop,
			PositionNmeaGgaFix:  &uacc.NmeaFix,
			CompassDegreesError: &uacc.CompassDegreeError,
		}, nil
	}
	return &pb.GetAccuracyResponse{
		Accuracy:            acc.AccuracyMap,
		PositionHdop:        &acc.Hdop,
		PositionVdop:        &acc.Vdop,
		PositionNmeaGgaFix:  &acc.NmeaFix,
		CompassDegreesError: &acc.CompassDegreeError,
	}, nil
}
