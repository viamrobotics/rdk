import { type Client, commonApi } from '@viamrobotics/sdk';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import { rcLogConditionally } from '@/lib/log';

export const doCommand = (client: Client, name: string, command: string) => {
  const request = new commonApi.DoCommandRequest();
  request.setName(name);
  request.setCommand(Struct.fromJavaScript(JSON.parse(command)));

  rcLogConditionally(request);

  return new Promise((resolve, reject) => {
    client.genericService.doCommand(request, (error, response) => (
      error ? reject(error) : resolve(response?.getResult()?.toObject())
    ));
  });
};
