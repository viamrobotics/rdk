import type { Client, ResponseStream, commonApi, robotApi } from '@viamrobotics/sdk';
import { components, resources, services, statuses } from '@/stores/resources';
import { currentWritable } from '@threlte/core';
import { StreamManager } from '@/lib/stream-manager';

const clientStores = {
  client: currentWritable<Client>(null!),
  components,
  connectionStatus: currentWritable<'idle' | 'connecting' | 'connected' | 'reconnecting'>('idle'),
  operations: currentWritable<{
    op: robotApi.Operation.AsObject;
    elapsed: number;
  }[]>([]),
  resources,
  rtt: currentWritable(0),
  sensorNames: currentWritable<commonApi.ResourceName.AsObject[]>([]),
  services,
  sessions: currentWritable<robotApi.Session.AsObject[]>([]),
  sessionsSupported: currentWritable(true),
  statuses,
  statusStream: currentWritable<null | ResponseStream<robotApi.StreamStatusResponse>>(null),
  streamManager: currentWritable<StreamManager>(null!),
} as const;

export const useClient = () => clientStores;
