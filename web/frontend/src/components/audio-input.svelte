<script lang='ts'>

import { StreamClient } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import type { ServiceError, Client } from '@viamrobotics/sdk';

export let name: string;
export let client: Client;

let audioInput = false;

const toggleExpand = async () => {
  audioInput = !audioInput;

  const streams = new StreamClient(client);

  if (audioInput) {
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
  <v-breadcrumbs
    slot="title"
    crumbs="audio_input"
  />
  <div class="h-auto border-x border-b border-black p-2">
    <div class="container mx-auto">
      <div class="pt-4">
        <div class="flex items-center gap-2">
          <v-switch
            id="audio-input"
            value={audioInput ? 'on' : 'off'}
            on:input={toggleExpand}
          />
          <span class="pr-2">Listen</span>
        </div>

        {#if audioInput}
        <div
          data-stream={name}
          class="clear-both h-fit transition-all duration-300 ease-in-out"
        />
        {/if}
      </div>
    </div>
  </div>
</v-collapse>
