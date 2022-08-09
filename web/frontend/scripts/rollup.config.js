/**
 * This file exists because protobuf cannot generate es modules, only commonjs...
 * but the rest of the world has moved on to es modules.
 * If in the future the first statement here becomes false, then please instead generate 
 * es modules and get rid of this.
 */

import commonjs from '@rollup/plugin-commonjs';
import { nodeResolve } from '@rollup/plugin-node-resolve';
import copy from 'rollup-plugin-copy';
import alias from '@rollup/plugin-alias';
 
const dir = './src/gen/proto';
const format = 'es';
const files = [
  'api/common/v1/common_pb',
  'api/component/arm/v1/arm_pb',
  'api/component/arm/v1/arm_pb_service',
  'api/component/base/v1/base_pb',
  'api/component/base/v1/base_pb_service',
  'api/component/board/v1/board_pb',
  'api/component/board/v1/board_pb_service',
  'api/component/camera/v1/camera_pb',
  'api/component/camera/v1/camera_pb_service',
  'api/component/gantry/v1/gantry_pb',
  'api/component/gantry/v1/gantry_pb_service',
  'api/component/generic/v1/generic_pb',
  'api/component/generic/v1/generic_pb_service',
  'api/component/gripper/v1/gripper_pb',
  'api/component/gripper/v1/gripper_pb_service',
  'api/component/imu/v1/imu_pb',
  'api/component/imu/v1/imu_pb_service',
  'api/component/inputcontroller/v1/input_controller_pb',
  'api/component/inputcontroller/v1/input_controller_pb_service',
  'api/component/motor/v1/motor_pb',
  'api/component/motor/v1/motor_pb_service',
  'api/component/servo/v1/servo_pb',
  'api/component/servo/v1/servo_pb_service',
  'api/robot/v1/robot_pb',
  'api/robot/v1/robot_pb_service',
  'api/service/motion/v1/motion_pb',
  'api/service/motion/v1/motion_pb_service',
  'api/service/navigation/v1/navigation_pb',
  'api/service/navigation/v1/navigation_pb_service',
  'api/service/sensors/v1/sensors_pb',
  'api/service/sensors/v1/sensors_pb_service',
  'api/service/slam/v1/slam_pb',
  'api/service/slam/v1/slam_pb_service',
  'api/service/vision/v1/vision_pb',
  'api/service/vision/v1/vision_pb_service',
  'stream/v1/stream_pb',
  'stream/v1/stream_pb_service',
];
 
const plugins = [
  nodeResolve(),
  commonjs(),
  alias({
    entries: [
      {
        find: '@improbable-eng/grpc-web',
        replacement: './node_modules/@improbable-eng/grpc-web/dist/grpc-web-client.js',
      },
    ],
  }),
];
 
export default files.map((file) => ({
  input: `${dir}/${file}.js`,
  output: {
    file: `${dir}/${file}.esm.js`,
    format,
  },
  plugins: [
    ...plugins,
    copy({
      targets: [{
        src: `${dir}/${file}.d.ts`,
        dest: dir,
        rename: () => `${file}.esm.d.ts`,
      }],
    }),
  ],
}));
