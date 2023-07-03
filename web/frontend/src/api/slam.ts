import { type Client, commonApi, slamApi } from '@viamrobotics/sdk';
import { grpc } from '@improbable-eng/grpc-web';
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

export const getPointCloudMap = (client: Client, name: string) => {
  const request = new slamApi.GetPointCloudMapRequest();
  request.setName(name);
  rcLogConditionally(request);

  const chunks: Uint8Array[] = [];
  const stream = client.slamService.getPointCloudMap(request);

  stream.on('data', (response) => {
    const chunk = response.getPointCloudPcdChunk_asU8();
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

export const getSLAMPosition = (client: Client, name: string) => {
  const request = new slamApi.GetPositionRequest();
  request.setName(name);

  return new Promise<commonApi.Pose | undefined>((resolve, reject) => {
    client.slamService.getPosition(request, new grpc.Metadata(), (error, response) => (
      error ? reject(error) : resolve(response?.getPose())
    ));
  });
};

