import { type Client, slamApi } from '@viamrobotics/sdk';

export const getPosition = async (robotClient: Client, name: string) => {
  const request = new slamApi.GetPositionRequest();
  request.setName(name);

  const response = await new Promise<slamApi.GetPositionResponse | null>((resolve, reject) => {
    robotClient.slamService.getPosition(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.getPose();
};
