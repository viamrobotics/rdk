<script lang='ts'>

import { StreamClient } from '@viamrobotics/sdk';
import type { Client, ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';

export let name: string;
export let client: Client;

let isOn = false;

const toggleExpand = async () => {
  isOn = !isOn

  const streams = new StreamClient(client);

  if (isOn) {
    try {
      await streams.add(name);
    } catch (error) {
      displayError(error as ServiceError);
    }
    return;
  }

  try {
    await streams.remove(name);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

</script>

<v-collapse title={name}>
  <v-breadcrumbs slot="title" crumbs="audio_input" />
  <div class="h-auto border border-t-0 border-medium p-2">
    <div class="container mx-auto">
      <div class="pt-4">
        <div class="flex items-center gap-2">
          <v-switch
            id="audio-input"
            value={isOn ? 'on' : 'off'}
            on:change={toggleExpand}
          />
          <span class="pr-2">Listen</span>
        </div>

        {#if isOn}
          <div
            data-stream={name}
            class="clear-both h-fit transition-all duration-300 ease-in-out"
          />
        {/if}
      </div>
    </div>
  </div>
</v-collapse>
