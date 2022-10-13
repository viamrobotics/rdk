
import { grpc } from '@improbable-eng/grpc-web';
import { streamApi, StreamServiceClient } from '../api';

export const addStream = async (name: string, streamService: StreamServiceClient) => {
  await new Promise<void>((resolve, reject) => {
    const req = new streamApi.AddStreamRequest();
    req.setName(name);
    streamService.addStream(req, new grpc.Metadata(), (err) => {
      if (err) {
        reject(err);
        return;
      }

      resolve();
    });
  });
};

export const removeStream = async (name: string, streamService: StreamServiceClient) => {
  await new Promise<void>((resolve, reject) => {
    const req = new streamApi.RemoveStreamRequest();
    req.setName(name);
    streamService.removeStream(req, new grpc.Metadata(), (err) => {
      if (err) {
        reject(err);
        return;
      }

      resolve();
    });
  });
};
