<script lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { Client, gantryApi } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { rcLogConditionally } from '@/lib/log';
import Collapse from '@/components/collapse.svelte';

export let name: string;
export let client: Client;
export let status: {
  is_moving: boolean
  lengths_mm: number[]
  positions_mm: number[]
} = {
  is_moving: false,
  lengths_mm: [],
  positions_mm: [],
};

$: parts = status.lengths_mm.map((_, index) => ({
  axis: index,
  pos: status.positions_mm[index]!,
  length: status.lengths_mm[index]!,
}))

$: if (status.lengths_mm.length !== status.positions_mm.length) {
  console.error('gantry lists different lengths');
}

const increment = (axis: number, amount: number) => {
  if (!status) {
    return;
  }

  const pos: number[] = [];

  for (let i = 0; i < parts.length; i += 1) {
    pos[i] = parts[i]!.pos;
  }

  pos[axis] += amount;

  const req = new gantryApi.MoveToPositionRequest();
  req.setName(name);
  req.setPositionsMmList(pos);

  rcLogConditionally(req);
  client.gantryService.moveToPosition(req, new grpc.Metadata(), displayError);
};

const stop = () => {
  const req = new gantryApi.StopRequest();
  req.setName(name);

  rcLogConditionally(req);
  client.gantryService.stop(req, new grpc.Metadata(), displayError);
};

</script>

<Collapse title={name}>
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
      label="Stop"
      on:click|stopPropagation={stop}
    />
  </div>
  <div class="overflow-auto border border-t-0 border-medium p-4">
    <table class="border border-t-0 border-medium p-4">
      <thead>
        <tr>
          <th class="border border-medium p-2">
            axis
          </th>
          <th
            class="border border-medium p-2"
            colspan="2"
          >
            position
          </th>
          <th class="border border-medium p-2">
            length
          </th>
        </tr>
      </thead>
      <tbody>
        {#each parts as part (part.axis)}
          <tr>
            <th class="border border-medium p-2">
              {part.axis}
            </th>
            <td class="flex gap-2 p-2">
              <v-button
                label="--"
                on:click={increment(part.axis, -10)}
              />
              <v-button
                label="-"
                on:click={increment(part.axis, -1)}
              />
              <v-button
                label="+"
                on:click={increment(part.axis, 1)}
              />
              <v-button
                label="++"
                on:click={increment(part.axis, 10)}
              />
            </td>
            <td class="border border-medium p-2">
              { part.pos.toFixed(2) }
            </td>
            <td class="border border-medium p-2">
              { part.length }
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</Collapse>
