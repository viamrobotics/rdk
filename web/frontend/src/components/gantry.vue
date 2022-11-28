<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { Client, gantryApi } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';

interface Props {
  name: string
  status: {
    parts: {
      pos: number
      axis: number
      length: number
    }[]
  }
  client: Client
}

const props = defineProps<Props>();

const increment = (axis: number, amount: number) => {
  const pos: number[] = [];
  for (let i = 0; i < props.status.parts.length; i += 1) {
    pos[i] = props.status.parts[i]!.pos;
  }
  pos[axis] += amount;

  const req = new gantryApi.MoveToPositionRequest();
  req.setName(props.name);
  req.setPositionsMmList(pos);
  props.client.gantryService.moveToPosition(req, new grpc.Metadata(), displayError);
};

const stop = () => {
  const request = new gantryApi.StopRequest();
  request.setName(props.name);
  props.client.gantryService.stop(request, new grpc.Metadata(), displayError);
};

</script>

<template>
  <v-collapse
    :title="name"
    class="gantry"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="gantry"
    />
    <div
      slot="header"
      class="flex items-center justify-between gap-2"
    >
      <v-button
        variant="danger"
        icon="stop-circle"
        label="STOP"
        @click.stop="stop"
      />
    </div>
    <div class="overflow-auto border border-t-0 border-black p-4">
      <table class="border border-t-0 border-black p-4">
        <thead>
          <tr>
            <th class="border border-black p-2">
              axis
            </th>
            <th
              class="border border-black p-2"
              colspan="2"
            >
              position
            </th>
            <th class="border border-black p-2">
              length
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="pp in status.parts"
            :key="pp.axis"
          >
            <th class="border border-black p-2">
              {{ pp.axis }}
            </th>
            <td class="flex p-2 gap-2">
              <v-button
                class="flex-nowrap"
                label="--"
                @click="increment(pp.axis, -10)"
              />
              <v-button
                label="-"
                @click="increment(pp.axis, -1)"
              />
              <v-button
                label="+"
                @click="increment(pp.axis, 1)"
              />
              <v-button
                label="++"
                @click="increment(pp.axis, 10)"
              />
            </td>
            <td class="border border-black p-2">
              {{ pp.pos.toFixed(2) }}
            </td>
            <td class="border border-black p-2">
              {{ pp.length }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </v-collapse>
</template>
