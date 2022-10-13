<script setup lang="ts">

import { onMounted } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { displayError } from '../lib/error';
import { createRobotService, robotApi, type RobotServiceClient } from '../api';

interface Props {
  operations: {
    op: robotApi.Operation.AsObject
    elapsed: number
  }[]
}

defineProps<Props>();

let robotService: RobotServiceClient;

const killOperation = (id: string) => {
  const req = new robotApi.CancelOperationRequest();
  req.setId(id);
  robotService.cancelOperation(req, new grpc.Metadata(), displayError);
};

onMounted(() => {
  robotService = createRobotService();
});

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
          v-for="{ op, elapsed } in operations"
          :key="op.id"
        >
          <td class="border border-black p-2">
            {{ op.id }}
          </td>
          <td class="border border-black p-2">
            {{ op.method }}
          </td>
          <td class="border border-black p-2">
            {{ elapsed }}ms
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
  </v-collapse>
</template>
