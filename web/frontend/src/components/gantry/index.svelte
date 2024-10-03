<script lang="ts">
import { useRobotClient } from '@/hooks/robot-client';
import Collapse from '@/lib/components/collapse.svelte';
import { displayError } from '@/lib/error';
import { rcLogConditionally } from '@/lib/log';
import { gantryApi } from '@viamrobotics/sdk';

export let name: string;
export let status: {
  is_moving: boolean;
  lengths_mm: number[];
  positions_mm: number[];
} = {
  is_moving: false,
  lengths_mm: [],
  positions_mm: [],
};

interface GantryStatus {
  pieces: {
    axis: number;
    pos: number;
    length: number;
  }[];
}

const { robotClient } = useRobotClient();

let modifyAllStatus: GantryStatus = {
  pieces: [],
};

let modifyAll = false;

$: parts = status.lengths_mm.map((_, index) => ({
  axis: index,
  pos: status.positions_mm[index]!,
  length: status.lengths_mm[index]!,
}));

$: if (status.lengths_mm.length !== status.positions_mm.length) {
  // eslint-disable-next-line no-console
  console.error('gantry lists different lengths');
}

const increment = (axis: number, amount: number) => {
  const pos: number[] = [];

  for (const [i, part] of parts.entries()) {
    pos[i] = part.pos;
  }

  if (pos[axis] == undefined) {
    pos[axis] = 0;
  }
  pos[axis] += amount;

  const req = new gantryApi.MoveToPositionRequest({
    name,
    positionsMm: pos,
  });

  rcLogConditionally(req);
  $robotClient.gantryService.moveToPosition(req).catch(displayError);
};

const gantryModifyAllDoMoveToPosition = () => {
  const pieces = modifyAllStatus.pieces.map((piece) => piece.pos);

  const req = new gantryApi.MoveToPositionRequest({
    name,
    positionsMm: pieces,
  });

  rcLogConditionally(req);
  $robotClient.gantryService.moveToPosition(req).catch(displayError);

  modifyAll = false;
};

const gantryHome = () => {
  const req = new gantryApi.HomeRequest({ name });

  rcLogConditionally(req);
  $robotClient.gantryService.home(req).catch(displayError);
};

const gantryModifyAll = () => {
  const nextPiece = [];

  for (const part of parts) {
    nextPiece.push({
      axis: part.axis,
      pos: part.pos,
      length: part.length,
    });
  }

  modifyAllStatus = {
    pieces: nextPiece,
  };
  modifyAll = true;
};

const stop = () => {
  const req = new gantryApi.StopRequest({ name });

  rcLogConditionally(req);
  $robotClient.gantryService.stop(req).catch(displayError);
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
      icon="stop-circle-outline"
      label="Stop"
      on:click|stopPropagation={stop}
    />
  </div>
  <div class="overflow-auto border border-t-0 border-medium p-4">
    <table class="border border-t-0 border-medium p-4">
      <thead>
        <tr>
          <th class="border border-medium p-2"> axis </th>
          <th
            class="border border-medium p-2"
            colspan="2"
          >
            position
          </th>
          <th class="border border-medium p-2"> length </th>
        </tr>
      </thead>
      <tbody>
        {#if modifyAll}
          {#each modifyAllStatus.pieces as piece, i (piece.axis)}
            <tr>
              <th class="border border-medium p-2">
                {parts[i]?.axis ?? 0}
              </th>
              <td class="border border-medium p-2">
                <input
                  type="number"
                  bind:value={piece.pos}
                  class="
                      h-[30px] w-full appearance-none border border-light bg-white px-2 py-1.5 pl-2.5
                      text-xs leading-tight outline-none hover:border-medium focus:border-gray-9
                    "
                />
              </td>
              <td class="border border-medium p-2">
                {parts[i]?.pos.toFixed(2) ?? 0}
              </td>
              <td class="border border-medium p-2">
                {parts[i]?.length ?? 0}
              </td>
            </tr>
          {/each}
        {:else}
          {#each parts as part (part.axis)}
            <tr>
              <th class="border border-medium p-2">
                {part.axis}
              </th>
              <td class="flex gap-2 p-2">
                <v-button
                  label="--"
                  on:click={() => increment(part.axis, -10)}
                />
                <v-button
                  label="-"
                  on:click={() => increment(part.axis, -1)}
                />
                <v-button
                  label="+"
                  on:click={() => increment(part.axis, 1)}
                />
                <v-button
                  label="++"
                  on:click={() => increment(part.axis, 10)}
                />
              </td>
              <td class="border border-medium p-2">
                {part.pos.toFixed(2)}
              </td>
              <td class="border border-medium p-2">
                {part.length}
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
    {#if modifyAll}
      <v-button
        icon="play-circle-filled"
        label="Go"
        class="mt-2 text-right"
        on:click={gantryModifyAllDoMoveToPosition}
      />
    {/if}
    <div class="mt-6 flex gap-2">
      {#if modifyAll}
        <v-button
          label="Cancel"
          on:click={() => {
            modifyAll = false;
          }}
        />
      {:else}
        <v-button
          label="Modify all"
          on:click={gantryModifyAll}
        />
        <v-button
          label="Home"
          on:click={gantryHome}
        />
      {/if}
    </div>
  </div>
</Collapse>
