import type { RobotClient } from '@viamrobotics/sdk';

const DO_COMMAND_CLIENT_TYPES = [
  'arm',
  'base',
  'board',
  'encoder',
  'gantry',
  'generic',
  'gripper',
  'input_controller',
  'motion',
  'motor',
  'movement_sensor',
  'navigation',
  'power_sensor',
  'sensors',
  'servo',
  'slam',
  'vision',
] as const;

type DoCommandClientType = (typeof DO_COMMAND_CLIENT_TYPES)[number];

type DoCommandServiceType =
  | 'armService'
  | 'baseService'
  | 'boardService'
  | 'encoderService'
  | 'gantryService'
  | 'genericService'
  | 'gripperService'
  | 'inputControllerService'
  | 'motionService'
  | 'motorService'
  | 'movementSensorService'
  | 'navigationService'
  | 'powerSensorService'
  | 'sensorsService'
  | 'servoService'
  | 'slamService'
  | 'visionService';

const CLIENT_TYPES: Record<DoCommandClientType, DoCommandServiceType> = {
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
};

export const getClientByType = (robotClient: RobotClient, type: string) => {
  const clientType = type as DoCommandClientType;
  if (!DO_COMMAND_CLIENT_TYPES.includes(clientType)) {
    return null;
  }

  return robotClient[CLIENT_TYPES[clientType]];
};
