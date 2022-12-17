<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { Client, robotApi } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';

interface Props {
  operations: {
    op: robotApi.Operation.AsObject
    elapsed: number
  }[],

  sessions: robotApi.Session.AsObject[],
  sessionsSupported: boolean,

  connectionManager: {
    rtt: number;
  },

  client: Client
}

const props = defineProps<Props>();

const killOperation = (id: string) => {
  const req = new robotApi.CancelOperationRequest();
  req.setId(id);

  rcLogConditionally(req);
  props.client.robotService.cancelOperation(req, new grpc.Metadata(), displayError);
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

<template>
  <v-collapse
    :title="sessionsSupported ? 'Operations & Sessions' : 'Operations'"
    class="operations"
  >
    <div class="border border-t-0 border-black p-4">
      <template
        v-if="connectionManager"
      >
        <div class="flex justify-end mb-4">
          <label>RTT:</label>
          <v-badge
            v-if="connectionManager.rtt < 50"
            variant="green"
            :label="connectionManager.rtt + ' ms'"
          />
          <v-badge
            v-else-if="connectionManager.rtt < 500"
            variant="orange"
            :label="connectionManager.rtt + ' ms'"
          />
          <v-badge
            v-else
            variant="red"
            :label="connectionManager.rtt + ' ms'"
          />
        </div>

        <div class="overflow-auto">
          <div class="font-bold p-2">
            Operations
          </div>
          <table class="w-full table-auto border border-black">
            <tr>
              <th class="border border-black p-2">
                id
              </th>
              <th class="border border-black p-2">
                session
              </th>
              <th class="border border-black p-2">
                method
              </th>
              <th class="border border-black p-2">
                elapsed time
              </th>
              <th class="border border-black p-2" />
            </tr>
            <tr
              v-for="{ op, elapsed } in operations"
              :key="op.id"
            >
              <td class="border border-black p-2">
                {{ op.id }} <span
                  v-if="client.sessionId === op.sessionId"
                  class="font-bold"
                >(this session)</span>
              </td>
              <td class="border border-black p-2">
                {{ op.sessionId || 'N/A' }}
              </td>
              <td class="border border-black p-2">
                {{ op.method }}
              </td>
              <td class="border border-black p-2">
                {{ elapsed }} ms
              </td>
              <td class="border border-black p-2 text-center">
                <v-button
                  label="Kill"
                  @click="killOperation(op.id)"
                />
              </td>
            </tr>
          </table>
        </div>

        <div
          v-if="sessionsSupported"
          class="overflow-auto"
        >
          <div class="font-bold p-2">
            Sessions
          </div>
          <table class="w-full table-auto border border-black">
            <tr>
              <th class="border border-black p-2">
                id
              </th>
              <th class="border border-black p-2">
                type
              </th>
              <th class="border border-black p-2">
                remote address
              </th>
              <th class="border border-black p-2">
                local address
              </th>
            </tr>
            <tr
              v-for="sess in sessions"
              :key="sess.id"
            >
              <td class="border border-black p-2">
                {{ sess.id }} <span
                  v-if="client.sessionId && sess.id === client.sessionId"
                  class="font-bold"
                >(ours)</span>
              </td>
              <td class="border border-black p-2">
                {{ peerConnectionType(sess.peerConnectionInfo) }}
              </td>
              <td class="border border-black p-2">
                {{ sess.peerConnectionInfo?.remoteAddress || 'N/A' }}
              </td>
              <td class="border border-black p-2">
                {{ sess.peerConnectionInfo?.localAddress || 'N/A' }}
              </td>
            </tr>
          </table>
        </div>
      </template>
    </div>
  </v-collapse>
</template>
