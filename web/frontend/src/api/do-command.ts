import {
  CameraClient,
  commonApi,
  doCommandFromClient,
  SensorClient,
  type ServiceError,
  type StructType,
} from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';
import type { grpc } from '@improbable-eng/grpc-web';

/*
 * TODO (DTCurrie): Callback, ServiceFunc, and DoCommandClient are all copy/paste from the
 * SDK. We should consider renaming them Callback and ServiceFunc to be more specific and
 * export them along with DoCommandClient to make `doCommandFromClient` more useable.
 */
type Callback<T> = (error: ServiceError | null, response: T | null) => void;

type ServiceFunc<Req, Resp> = (
  request: Req,
  metadata: grpc.Metadata,
  callback: Callback<Resp>
) => void;

export interface DoCommandClient {
  doCommand: ServiceFunc<
    commonApi.DoCommandRequest,
    commonApi.DoCommandResponse
  >;
}

export const doCommand = async (
  client: DoCommandClient | SensorClient | CameraClient,
  name: string,
  command: string
) => {
  const parsedCommand = JSON.parse(command) as StructType;

  rcLogConditionally({
    request: 'DoCommandRequest',
    client,
    name,
    command: parsedCommand,
  });

  // TODO(RSDK-7272): Figure out long-term solution for DoCommand in RC
  if (client instanceof SensorClient || client instanceof CameraClient) {
    return client.doCommand(parsedCommand);
  }
  return doCommandFromClient(client, name, parsedCommand);
};
