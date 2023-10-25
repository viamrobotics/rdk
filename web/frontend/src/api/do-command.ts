import { type Client, commonApi } from '@viamrobotics/sdk';
import { Struct, type JavaScriptValue } from 'google-protobuf/google/protobuf/struct_pb';
import { rcLogConditionally } from '@/lib/log';

export const doCommand = async (robotClient: Client, name: string, command: string) => {
  const request = new commonApi.DoCommandRequest();
  request.setName(name);
  request.setCommand(Struct.fromJavaScript(JSON.parse(command) as Record<string, JavaScriptValue>));

  rcLogConditionally(request);

  const response = await new Promise<commonApi.DoCommandResponse | null>((resolve, reject) => {
    robotClient.genericService.doCommand(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.getResult()?.toObject();
};
