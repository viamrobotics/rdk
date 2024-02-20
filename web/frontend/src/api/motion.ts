import { type Client, commonApi, motionApi } from '@viamrobotics/sdk';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import { getPosition } from './slam';
type ResourceName = commonApi.ResourceName.AsObject;

export const moveOnMap = async (
  robotClient: Client,
  slamServiceName: ResourceName,
  componentName: ResourceName,
  x: number,
  y: number
): Promise<string | undefined> => {
  const request = new motionApi.MoveOnMapRequest();
  /*
   * here we set the name of the motion service the user is using
   */
  request.setName('builtin');

  // set pose in frame
  const lastPose = await getPosition(robotClient, slamServiceName.name);

  const destination = new commonApi.Pose();
  destination.setX(x * 1000);
  destination.setY(y * 1000);
  destination.setZ(0);
  destination.setOX(lastPose!.getOX());
  destination.setOY(lastPose!.getOY());
  destination.setOZ(lastPose!.getOZ());
  destination.setTheta(lastPose!.getTheta());
  request.setDestination(destination);

  // set SLAM resource name
  const slamResourceName = new commonApi.ResourceName();
  slamResourceName.setNamespace(slamServiceName.namespace);
  slamResourceName.setType(slamServiceName.type);
  slamResourceName.setSubtype(slamServiceName.subtype);
  slamResourceName.setName(slamServiceName.name);
  request.setSlamServiceName(slamResourceName);

  // set the motion configuration
  const motionCfg = new motionApi.MotionConfiguration();
  motionCfg.setPlanDeviationM(0.5);
  request.setMotionConfiguration(motionCfg);

  // set component name
  const baseResourceName = new commonApi.ResourceName();
  baseResourceName.setNamespace(componentName.namespace);
  baseResourceName.setType(componentName.type);
  baseResourceName.setSubtype(componentName.subtype);
  baseResourceName.setName(componentName.name);
  request.setComponentName(baseResourceName);

  // set extra as position-only constraint
  request.setExtra(
    Struct.fromJavaScript({
      motion_profile: 'position_only',
    })
  );

  const response = await new Promise<motionApi.MoveOnMapResponse | null>(
    (resolve, reject) => {
      robotClient.motionService.moveOnMap(request, (error, res) => {
        if (error) {
          reject(error);
        } else {
          resolve(res);
        }
      });
    }
  );

  return response?.getExecutionId();
};
