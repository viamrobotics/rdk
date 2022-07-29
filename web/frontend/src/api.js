import commonApi from './gen/proto/api/common/v1/common_pb.esm';
import armApi from './gen/proto/api/component/arm/v1/arm_pb.esm';
import baseApi from './gen/proto/api/component/base/v1/base_pb.esm';
import cameraApi from './gen/proto/api/component/camera/v1/camera_pb.esm';
import gripperApi from './gen/proto/api/component/gripper/v1/gripper_pb.esm';
import robotApi from './gen/proto/api/robot/v1/robot_pb.esm';
import sensorsApi from './gen/proto/api/service/sensors/v1/sensors_pb.esm';
import servoApi from './gen/proto/api/component/servo/v1/servo_pb.esm';
import streamApi from './gen/proto/stream/v1/stream_pb.esm';
import motorApi from './gen/proto/api/component/motor/v1/motor_pb.esm';

/**
 * Every window variable on this page is being currently used by the blockly page in App.
 * Once we switch blockly to using import / export we should remove / clean up these window variables.
 */
window.commonApi = commonApi;
window.armApi = armApi;
window.baseApi = baseApi;
window.cameraApi = cameraApi;
window.gripperApi = gripperApi;
window.sensorsApi = sensorsApi;
window.servoApi = servoApi;
window.streamApi = streamApi;
window.motorApi = motorApi;
/**
 * This window variable is used by the config page to access the discovery service.
 * As with variables above, once we switch to using import / export we should
 * remove / clean up these window variables.
 */
window.robotApi = robotApi;
