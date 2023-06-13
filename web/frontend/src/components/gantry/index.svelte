<script lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { Client, gantryApi } from '@viamrobotics/sdk';
import { displayError } from '../../lib/error';
import { rcLogConditionally } from '../../lib/log';

export let name:string;
export let status: {
  parts: {
    pos:number
    axis:number
    length:number
  }[]
};
export let client:Client;

const increment = (axis: number, amount: number) => {
  const pos: number[] = [];
  for (let i = 0; i < status.parts.length; i += 1) {
    pos[i] = status.parts[i]!.pos;
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

<v-collapse
title={name}
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
        {#each status.parts as part}
        <tr>
        <th class="border border-medium p-2">
            { part.axis }
        </th>
        <td class="flex gap-2 p-2">
            <v-button
            class="flex-nowrap"
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
</v-collapse>
