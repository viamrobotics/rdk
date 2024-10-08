import {
  ArmClient,
  BaseClient,
  BoardClient,
  CameraClient,
  EncoderClient,
  GantryClient,
  GenericComponentClient,
  GripperClient,
  InputControllerClient,
  MotionClient,
  MotorClient,
  MovementSensorClient,
  NavigationClient,
  PowerSensorClient,
  SensorClient,
  ServoClient,
  SlamClient,
  VisionClient,
  type Resource,
  type RobotClient,
} from '@viamrobotics/sdk';

const CLIENT_TYPES = {
  arm: ArmClient,
  base: BaseClient,
  board: BoardClient,
  camera: CameraClient,
  encoder: EncoderClient,
  gantry: GantryClient,
  generic: GenericComponentClient,
  gripper: GripperClient,
  input_controller: InputControllerClient,
  motion: MotionClient,
  motor: MotorClient,
  movement_sensor: MovementSensorClient,
  navigation: NavigationClient,
  power_sensor: PowerSensorClient,
  sensor: SensorClient,
  servo: ServoClient,
  slam: SlamClient,
  vision: VisionClient,
} as const;

export const getClientByType = (
  robotClient: RobotClient,
  type: string,
  name: string
): Resource | undefined => {
  if (Object.hasOwn(CLIENT_TYPES, type)) {
    return new CLIENT_TYPES[type as keyof typeof CLIENT_TYPES](
      robotClient,
      name
    );
  }
  return undefined;
};
