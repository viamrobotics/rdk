
import { grpc } from '@improbable-eng/grpc-web';
import { Client, streamApi } from '@viamrobotics/sdk';

export const addStream = async (client: Client, name: string) => {
  await new Promise<void>((resolve, reject) => {
    const req = new streamApi.AddStreamRequest();
    req.setName(name);
    client.streamService.addStream(req, new grpc.Metadata(), (err) => {
      if (err) {
        reject(err);
        return;
      }

      resolve();
    });
  });
};

export const removeStream = async (client: Client, name: string) => {
  await new Promise<void>((resolve, reject) => {
    const req = new streamApi.RemoveStreamRequest();
    req.setName(name);
    client.streamService.removeStream(req, new grpc.Metadata(), (err) => {
      if (err) {
        reject(err);
        return;
      }

      resolve();
    });
  });
};
