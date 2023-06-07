import { type Client, commonApi, motionApi, robotApi } from '@viamrobotics/sdk';
import { grpc } from '@improbable-eng/grpc-web';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import { getSLAMPosition } from './slam';
import { rcLogConditionally } from '@/lib/log';

export interface Pose {
  x: number
  y: number
  z: number
  ox: number
  oy: number
  oz: number
  th: number
}

export const moveOnMap = async (client: Client, name: string, componentName: string, x: number, y: number) => {
  const request = new motionApi.MoveOnMapRequest();

  /*
   * here we set the name of the motion service the user is using
   */
  request.setName('builtin');

  // set pose in frame
  const lastPose = await getSLAMPosition(client, name);

  const destination = new commonApi.Pose();
  destination.setX(x);
  destination.setY(y);
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

  return new Promise((resolve, reject) => {
    client.motionService.moveOnMap(request, new grpc.Metadata(), (error, response) => (
      error ? reject(error) : resolve(response?.getSuccess())
    ));
  });
};

export const stopMoveOnMap = (client: Client, operations: { op: robotApi.Operation.AsObject }[]) => {
  return new Promise((resolve, reject) => {
    for (const { op } of operations) {
      if (op.method.includes('MoveOnMap')) {
        const req = new robotApi.CancelOperationRequest();
        req.setId(op.id);
        rcLogConditionally(req);
        client.robotService.cancelOperation(req, new grpc.Metadata(), (error, response) => (
          error ? reject(error) : resolve(response)
        ));
        return;
      }
    }

    reject(new Error('Operation not found!'));
  });
};
