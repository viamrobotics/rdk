import { type Client, slamApi } from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';

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

export const getPointCloudMap = (robotClient: Client, name: string) => {
  const request = new slamApi.GetPointCloudMapRequest();
  request.setName(name);
  rcLogConditionally(request);

  const chunks: Uint8Array[] = [];
  const stream = robotClient.slamService.getPointCloudMap(request);

  stream.on('data', (response) => {
    const chunk = response.getPointCloudPcdChunk_asU8();
    console.log(chunk)
    chunks.push(chunk);
  });

  return new Promise<Uint8Array>((resolve, reject) => {
    stream.on('status', (status) => {
      if (status.code !== 0) {
        const error = {
          message: status.details,
          code: status.code,
          metadata: status.metadata,
        };
        reject(error);
      }
    });

    stream.on('end', (end) => {
      if (end === undefined) {
        const error = { message: 'Stream ended without status code' };
        reject(error);
      } else if (end.code !== 0) {
        const error = {
          message: end.details,
          code: end.code,
          metadata: end.metadata,
        };
        reject(error);
      }
      const arr = concatArrayU8(chunks);
      resolve(arr);
    });
  });
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

export const getLatestMapInfo = async (robotClient: Client, name: string) => {
  const request = new slamApi.GetLatestMapInfoRequest();
  request.setName(name);
  const response = await new Promise<slamApi.GetLatestMapInfoResponse | null >((resolve, reject) => {
    robotClient.slamService.getLatestMapInfo(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });
  return response?.getLastMapUpdate();
};

