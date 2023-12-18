import { type Client, commonApi, motionApi, robotApi } from '@viamrobotics/sdk';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import { getPosition } from './slam';
import { rcLogConditionally } from '@/lib/log';

export const moveOnMap = async (robotClient: Client, name: string, componentName: string, x: number, y: number) => {
  const req = new motionApi.NewMoveOnMapNewRequest();

  const request = new motionApi.MoveOnMapRequest();

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
  const baseResourceName = new commonApi.ResourceName();
  baseResourceName.setNamespace('rdk');
  baseResourceName.setType('component');
  baseResourceName.setSubtype('base');
  baseResourceName.setName(componentName);
  request.setComponentName(baseResourceName);

  // set extra as position-only constraint
  request.setExtra(
    Struct.fromJavaScript({
      motion_profile: 'position_only',
    })
  );

  const response = await new Promise<motionApi.MoveOnMapResponse | null>((resolve, reject) => {
    robotClient.motionService.moveOnMap(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.getSuccess();
};

export const stopMoveOnMap = async (robotClient: Client, operations: { op: robotApi.Operation.AsObject }[]) => {
  const match = operations.find(({ op }) => op.method.includes('MoveOnMap'));

  if (!match) {
    throw new Error('Operation not found!');
  }

  const req = new robotApi.CancelOperationRequest();
  req.setId(match.op.id);
  rcLogConditionally(req);

  const response = await new Promise<robotApi.CancelOperationResponse | null>((resolve, reject) => {
    robotClient.robotService.cancelOperation(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject();
};
