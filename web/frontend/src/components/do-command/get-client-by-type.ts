import type { RobotClient } from '@viamrobotics/sdk';

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

export const getClientByType = (robotClient: RobotClient, type: string) => {
  if (!Object.hasOwn(CLIENT_TYPES, type)) {
    return null;
  }

  return robotClient[CLIENT_TYPES[type as keyof typeof CLIENT_TYPES]];
};
