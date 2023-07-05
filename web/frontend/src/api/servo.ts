import { type Client, servoApi } from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';

export const move = async (robotClient: Client, name: string, angle: number) => {
  const request = new servoApi.MoveRequest();
  request.setName(name);
  request.setAngleDeg(angle);

  rcLogConditionally(request);

  const response = await new Promise<servoApi.MoveResponse | null>((resolve, reject) => {
    robotClient.servoService.move(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject();
};
