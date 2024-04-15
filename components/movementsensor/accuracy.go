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
// used by the client.
func protoFeaturesToAccuracy(resp *pb.GetAccuracyResponse) *Accuracy {
	uacc := UnimplementedOptionalAccuracies()
	if resp == nil {
		return UnimplementedOptionalAccuracies()
	}

	hdop := resp.PositionHdop
	if hdop == nil {
		hdop = &uacc.Hdop
	}

	vdop := resp.PositionVdop
	if vdop == nil {
		vdop = &uacc.Vdop
	}

	compass := resp.CompassDegreesError
	if compass == nil {
		compass = &uacc.CompassDegreeError
	}

	nmeaFix := resp.PositionNmeaGgaFix
	if nmeaFix == nil {
		nmeaFix = &uacc.NmeaFix
	}

	return &Accuracy{
		AccuracyMap:        resp.Accuracy,
		Hdop:               *hdop,
		Vdop:               *vdop,
		NmeaFix:            *nmeaFix,
		CompassDegreeError: *compass,
	}
}

// AccuracyToProtoResponse converts an Accuracy struct into a protobuf GetAccuracyResponse.
// used by the server.
func accuracyToProtoResponse(acc *Accuracy) (*pb.GetAccuracyResponse, error) {
	uacc := UnimplementedOptionalAccuracies()
	if acc == nil {
		return &pb.GetAccuracyResponse{
			Accuracy:            map[string]float32{},
			PositionHdop:        &uacc.Hdop,
			PositionVdop:        &uacc.Vdop,
			CompassDegreesError: &uacc.CompassDegreeError,
			// default value of the GGA NMEA Fix when Accuracy struct is nil is -1 - a meaningless value in terms of GGA Fixes.
			PositionNmeaGgaFix: &uacc.NmeaFix,
		}, nil
	}

	hdop := uacc.Hdop
	if acc.Hdop > 0 {
		hdop = acc.Hdop
	}

	vdop := uacc.Vdop
	if acc.Vdop > 0 {
		vdop = acc.Vdop
	}

	compass := uacc.CompassDegreeError
	if acc.CompassDegreeError > 0 {
		compass = acc.CompassDegreeError
	}

	return &pb.GetAccuracyResponse{
		Accuracy:            acc.AccuracyMap,
		PositionHdop:        &hdop,
		PositionVdop:        &vdop,
		CompassDegreesError: &compass,
		// default value of the GGA NMEA Fix when Accuracy struct is non-nil is 0 - invalid GGA Fix.
		PositionNmeaGgaFix: &acc.NmeaFix,
	}, nil
}
