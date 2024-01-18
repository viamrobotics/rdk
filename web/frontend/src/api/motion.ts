import { type Client, commonApi, motionApi } from '@viamrobotics/sdk';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import { getPosition } from './slam';
import type { ResourceName } from '@viamrobotics/sdk/dist/gen/common/v1/common_pb';

export const moveOnMap = async (robotClient: Client, name: string, componentName: string, x: number, y: number): Promise<string | undefined> =>  {
  const request = new motionApi.MoveOnMapNewRequest();

  /*
   * here we set the name of the motion service the user is using
   */
  request.setName('builtin');

  // set pose in frame
  const lastPose = await getPosition(robotClient, name);

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
  slamResourceName.setNamespace('rdk');
  slamResourceName.setType('service');
  slamResourceName.setSubtype('slam');
  slamResourceName.setName(name);
  request.setSlamServiceName(slamResourceName);

  // set component name
  request.setComponentName(namedBase(componentName));

  // set extra as position-only constraint
  request.setExtra(
    Struct.fromJavaScript({
      motion_profile: 'position_only',
    })
  );

  const response = await new Promise<motionApi.MoveOnMapNewResponse | null>((resolve, reject) => {
    robotClient.motionService.moveOnMapNew(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.getExecutionId()
};

const namedBase = (componentName: string): ResourceName => {
  const baseResourceName = new commonApi.ResourceName();
  baseResourceName.setNamespace('rdk');
  baseResourceName.setType('component');
  baseResourceName.setSubtype('base');
  baseResourceName.setName(componentName);
  return baseResourceName
}

export const stopMoveOnMap = async (robotClient: Client, componentName: string) => {
  const request = new motionApi.StopPlanRequest();
  // TODO: This needs to be the actual name of the motion service
  request.setName('builtin');
  request.setComponentName(namedBase(componentName));

  await new Promise<motionApi.StopPlanResponse | null>((resolve, reject) => {
    robotClient.motionService.stopPlan(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });
}