import { grpc } from '@improbable-eng/grpc-web';
import { type Client, servoApi } from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';

export const move = (client: Client, name: string, angle: number) => {
  const request = new servoApi.MoveRequest();
  request.setName(name);
  request.setAngleDeg(angle);

  rcLogConditionally(request);

  return new Promise((resolve, reject) => {
    client.servoService.move(request, new grpc.Metadata(), (error) => {
      if (error) {
        return reject(error);
      }

      resolve(null);
    });
  });
};
