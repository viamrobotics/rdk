import type { Client, ResponseStream, commonApi, robotApi } from '@viamrobotics/sdk';
import { components, resources, services, statuses } from '@/stores/resources';
import { currentWritable } from '@threlte/core';
import { StreamManager } from '@/lib/stream-manager';
import { onMount } from 'svelte';

const context = {
  robotClient: currentWritable<Client>(null!),
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

export const useRobotClient = () => context;

export type DisconnectCallback = () => void
export type ConnectCallback = (() => DisconnectCallback) | (() => void)

/**
 * Pass a callback to this hook that will fire whenever an initial connection or reconnect occurs.
 * 
 * The callback can return a disconnection callback that will fire whenever a disconnect or unmount occurs.
 * 
 * @example
 * ```ts
 * useConnect(() => {
 *    const clearPoll = startPolling()
 *    return () => clearPoll()
 * })
 * ```
 */
export const useConnect = (callback: ConnectCallback) => {
  const { connectionStatus } = useRobotClient();

  // eslint-disable-next-line @typescript-eslint/no-invalid-void-type
  let disconnectCallback: DisconnectCallback | void

  onMount(() => {
    const unsubscribe = connectionStatus.subscribe((value) => {
      if (value === 'connected') {
        disconnectCallback = callback();
      } else if (value === 'reconnecting') {
        disconnectCallback?.()
      }
    })

    return () => {
      unsubscribe()
      disconnectCallback?.()
    }
  });
};
