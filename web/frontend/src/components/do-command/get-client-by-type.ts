import {
  SensorClient,
  type RobotClient,
  CameraClient,
} from '@viamrobotics/sdk';

const CLIENT_TYPES = {
  arm: 'armService',
  base: 'baseService',
  board: 'boardService',
  encoder: 'encoderService',
  gantry: 'gantryService',
  generic: 'genericService',
  gripper: 'gripperService',
  input_controller: 'inputControllerService',
  motion: 'motionService',
  motor: 'motorService',
  movement_sensor: 'movementSensorService',
  navigation: 'navigationService',
  power_sensor: 'powerSensorService',
  sensors: 'sensorsService',
  servo: 'servoService',
  slam: 'slamService',
  vision: 'visionService',
} as const;

export const getClientByType = (
  robotClient: RobotClient,
  type: string,
  name: string
) => {
  if (Object.hasOwn(CLIENT_TYPES, type)) {
    return robotClient[CLIENT_TYPES[type as keyof typeof CLIENT_TYPES]];
  }

  // TODO(RSDK-7272): Figure out long-term solution for DoCommand in RC
  if (type === 'sensor') {
    return new SensorClient(robotClient, name);
  } else if (type === 'camera') {
    return new CameraClient(robotClient, name);
  }

  return null;
};
