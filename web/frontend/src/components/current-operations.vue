<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import robotApi from '../gen/proto/api/robot/v1/robot_pb.esm';
import { displayError } from '../lib/error';

interface Props {
  operations: {
    id: string
    method: string
    elapsed: number
  }[]
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
          v-for="operation in operations"
          :key="operation.id"
        >
          <td class="border border-black p-2">
            {{ operation.id }}
          </td>
          <td class="border border-black p-2">
            {{ operation.method }}
          </td>
          <td class="border border-black p-2">
            {{ operation.elapsed }}ms
          </td>
          <td class="border border-black p-2 text-center">
            <v-button
              label="Kill"
              @click="killOperation(operation.id)"
            />
          </td>
        </tr>
      </table>
    </div>
  </v-collapse>
</template>
