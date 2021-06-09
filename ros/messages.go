package ros

// L515Message reflects the JSON data format for rosbag Intel Realsense data
type L515Message struct {
	Meta struct {
		Secs  int
		Nsecs int
	}
	Data struct {
		Layout struct {
			Dim []struct {
				Label  string
				Size   int
				Stride int
			}
			DataOffset int `json:"data_offset"`
		}
		Data []byte
	}
}

// ImuMessage reflects the JSON data format for rosbag imu data
type ImuMessage struct {
	Meta struct {
		Secs  int
		Nsecs int
	}
	Data struct {
		Header struct {
			Seq   int
			Stamp struct {
				Secs  int
				Nsecs int
			}
			FrameID string `json:"frame_id"`
		}
		Orientation struct {
			X float64
			Y float64
			Z float64
			W float64
		}
		OrientationCovariance [9]int `json:"orientation_covariance"`
		AngularVelocity       struct {
			X float64
			Y float64
			Z float64
		} `json:"angular_velocity"`
		AngularVelocityCovariance [9]int `json:"angular_velocity_covariance"`
		LinearAcceleration        struct {
			X float64
			Y float64
			Z float64
		} `json:"linear_acceleration"`
		LinearAccelerationCovariance [9]int `json:"linear_acceleration_covariance"`
	}
}
