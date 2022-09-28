# ROS package
The ROS package implements functionality that bridges the gap between `rdk` and `ROS`.

## Plane Segmentation
Plane segmentation works with ROS bags that contain the `/L515_ImageWithDepth` rostopic that publishes Intel Realsense L515 RGBD data.

It saves png images of the rgbd data, as well as segmented planes.

Run `rosbag_parser/cmd`:
```bash
go run rosbag_parser/cmd/main.go <path_to_your_rosbag>
```