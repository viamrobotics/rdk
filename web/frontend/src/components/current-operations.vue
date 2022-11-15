<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import robotApi, { type Operation } from '../gen/robot/v1/robot_pb.esm';
import { displayError } from '../lib/error';

interface Props {
  operations: {
    op: Operation.AsObject
    elapsed: number
  }[],

  connectionManager: {
    rtt: number;
  },

  sessionOps: Set<string>
}

defineProps<Props>();

const killOperation = (id: string) => {
  const req = new robotApi.CancelOperationRequest();
  req.setId(id);
  window.robotService.cancelOperation(req, new grpc.Metadata(), displayError);
};

</script>

<template>
  <v-collapse
    title="Current Operations"
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
          <table class="w-full table-auto border border-black">
            <tr>
              <th class="border border-black p-2">
                id
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
                  v-if="sessionOps.has(op.id)"
                  class="font-bold"
                >(this session)</span>
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
      </template>
    </div>
  </v-collapse>
</template>
