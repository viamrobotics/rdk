
import { grpc } from '@improbable-eng/grpc-web';
import { streamApi } from '@viamrobotics/sdk';

export const addStream = async (name: string) => {
  await new Promise<void>((resolve, reject) => {
    const req = new streamApi.AddStreamRequest();
    req.setName(name);
    window.streamService.addStream(req, new grpc.Metadata(), (err) => {
      if (err) {
        reject(err);
        return;
      }

      resolve();
    });
  });
};

export const removeStream = async (name: string) => {
  await new Promise<void>((resolve, reject) => {
    const req = new streamApi.RemoveStreamRequest();
    req.setName(name);
    window.streamService.removeStream(req, new grpc.Metadata(), (err) => {
      if (err) {
        reject(err);
        return;
      }

      resolve();
    });
  });
};
