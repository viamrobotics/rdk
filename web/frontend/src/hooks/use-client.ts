import type { commonApi, robotApi } from '@viamrobotics/sdk';
import { client } from '@/stores/client';
import { components, resources, services, statuses } from '@/stores/resources';
import { statusStream } from '@/stores/streams';
import { currentWritable } from '@threlte/core';
import { StreamManager } from '@/lib/stream-manager';

const clientStores = {
  client,
  components,
  connectionStatus: currentWritable<'idle' | 'connecting' | 'connected' | 'reconnecting'>('idle'),
  operations: currentWritable<{
    op: robotApi.Operation.AsObject;
    elapsed: number;
  }[]>([]),
  resources,
  rtt: currentWritable(0),
  sessions: currentWritable<robotApi.Session.AsObject[]>([]),
  sessionsSupported: currentWritable(true),
  sensorNames: currentWritable<commonApi.ResourceName.AsObject[]>([]),
  services,
  statuses,
  statusStream,
  streamManager: currentWritable<StreamManager>(null!),
} as const;

export const useClient = () => clientStores;
