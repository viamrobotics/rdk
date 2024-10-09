import {
  type Client,
  commonApi,
  motionApi,
  MotionClient,
  ResourceName,
  Struct,
} from '@viamrobotics/sdk';
import { getPosition } from './slam';

export const moveOnMap = async (
  robotClient: Client,
  slamServiceName: ResourceName,
  componentName: ResourceName,
  x: number,
  y: number
): Promise<string | undefined> => {
  const lastPose = await getPosition(robotClient, slamServiceName.name);

  const destination = new commonApi.Pose({
    x: x * 1000,
    y: y * 1000,
    z: 0,
    oX: lastPose?.oX,
    oY: lastPose?.oY,
    oZ: lastPose?.oZ,
    theta: lastPose?.theta,
  });

  const slamResourceName = new ResourceName({
    namespace: slamServiceName.namespace,
    type: slamServiceName.type,
    subtype: slamServiceName.subtype,
    name: slamServiceName.name,
  });

  const motionCfg = new motionApi.MotionConfiguration({
    planDeviationM: 0.5,
  });

  const baseResourceName = new ResourceName({
    namespace: componentName.namespace,
    type: componentName.type,
    subtype: componentName.subtype,
    name: componentName.name,
  });

  // set extra as position-only constraint
  const extra = Struct.fromJson({
    motion_profile: 'position_only',
  });

  const client = new MotionClient(robotClient, 'builtin');
  return client.moveOnMap(
    destination,
    baseResourceName,
    slamResourceName,
    motionCfg,
    [],
    extra
  );
};
