import { type Client, slamApi, SlamClient } from '@viamrobotics/sdk';

const concatArrayU8 = (arrays: Uint8Array[]) => {
  const totalLength = arrays.reduce((acc, value) => acc + value.length, 0);
  const result = new Uint8Array(totalLength);

  let length = 0;

  for (const array of arrays) {
    result.set(array, length);
    length += array.length;
  }

  return result;
};

export const getPointCloudMap = async (slamClient: SlamClient) => {
  const chunks = await slamClient.getPointCloudMap();
  return concatArrayU8(chunks);
};

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
