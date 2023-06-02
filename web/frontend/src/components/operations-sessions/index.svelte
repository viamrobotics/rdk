<script lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { Client, robotApi } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { rcLogConditionally } from '@/lib/log';

export let operations: { op: robotApi.Operation.AsObject; elapsed: number }[];
export let sessions: robotApi.Session.AsObject[];
export let sessionsSupported: boolean;
export let connectionManager: { rtt: number };
export let client: Client;

const killOperation = (id: string) => {
  const req = new robotApi.CancelOperationRequest();
  req.setId(id);

  rcLogConditionally(req);
  client.robotService.cancelOperation(req, new grpc.Metadata(), displayError);
};


const peerConnectionType = (info?: robotApi.PeerConnectionInfo.AsObject) => {
  if (!info) {
    return 'N/A';
  }

  switch (info.type) {
    case robotApi.PeerConnectionType.PEER_CONNECTION_TYPE_GRPC: {
      return 'gRPC';
    }
    case robotApi.PeerConnectionType.PEER_CONNECTION_TYPE_WEBRTC: {
      return 'WebRTC';
    }
    default: {
      return 'Unknown';
    }
  }
};

</script>

<v-collapse title={sessionsSupported ? 'Operations & Sessions' : 'Operations'}>
  <div class="border border-t-0 border-medium p-4 text-xs">
    <div class="mb-4 flex gap-2 justify-end items-center">
      <label>RTT:</label>
      {#if connectionManager.rtt < 50}
        <v-badge
          variant="green"
          label={`${connectionManager.rtt} ms`}
        />
      {:else if connectionManager.rtt < 500}
        <v-badge
          variant="orange"
          label={`${connectionManager.rtt} ms`}
        />
      {:else}
        <v-badge
          variant="red"
          label={`${connectionManager.rtt} ms`}
        />
      {/if}
    </div>

    <div class="overflow-auto">
      <div class="p-2 font-bold">Operations</div>
      <table class="w-full table-auto border border-medium">
        <tr>
          <th class="border border-medium p-2">id</th>
          <th class="border border-medium p-2">session</th>
          <th class="border border-medium p-2">method</th>
          <th class="border border-medium p-2">elapsed time</th>
          <th class="border border-medium p-2" />
        </tr>
        {#each operations as { op, elapsed } (op.id)}
          <tr>
            <td class="border border-medium p-2">
              {op.id}
              {#if client.sessionId === op.sessionId}
                <span class="font-bold">(this session)</span>
              {/if}
            </td>
            <td class="border border-medium p-2">{op.sessionId || 'N/A'}</td>
            <td class="border border-medium p-2">{op.method}</td>
            <td class="border border-medium p-2">{elapsed} ms</td>
            <td class="border border-medium p-2 text-center">
              <v-button label="Kill" on:click={() => killOperation(op.id)} />
            </td>
          </tr>
        {/each}
      </table>
    </div>

    {#if sessionsSupported}
      <div class="overflow-auto">
        <div class="p-2 font-bold">Sessions</div>
        <table class="w-full table-auto border border-medium">
          <tr>
            <th class="border border-medium p-2">id</th>
            <th class="border border-medium p-2">type</th>
            <th class="border border-medium p-2">remote address</th>
            <th class="border border-medium p-2">local address</th>
          </tr>
          {#each sessions as session (session.id)}
            <tr>
              <td class="border border-medium p-2">
                {session.id}
                {#if client.sessionId && session.id === client.sessionId}
                  <span class="font-bold">(ours)</span>
                {/if}
              </td>
              <td class="border border-medium p-2">
                {peerConnectionType(session.peerConnectionInfo)}
              </td>
              <td class="border border-medium p-2">
                {session.peerConnectionInfo?.remoteAddress || 'N/A'}
              </td>
              <td class="border border-medium p-2">
                {session.peerConnectionInfo?.localAddress || 'N/A'}
              </td>
            </tr>
          {/each}
        </table>
      </div>
    {/if}
  </div>
</v-collapse>
