import { client } from '@/stores/client';
import { resources } from '@/stores/resources';
import { statusStream } from '@/stores/streams';
import { currentWritable } from '@threlte/core';

const stores = {
  client,
  connectionStatus: currentWritable<'idle' | 'connecting' | 'connected' | 'disconnected'>('idle'),
  operations: currentWritable<any[]>([]),
  resources,
  rtt: currentWritable(0),
  sessions: currentWritable<any[]>([]),
  sessionsSupported: currentWritable(true),
  statusStream,
};

export const useClient = () => stores;
