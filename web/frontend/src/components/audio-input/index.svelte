<script lang='ts'>

import { StreamClient } from '@viamrobotics/sdk';
import type { Client, ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import Collapse from '@/components/collapse.svelte';

export let name: string;
export let client: Client;

let audio: HTMLAudioElement;

let isOn = false;

const toggleExpand = async () => {
  isOn = !isOn

  const streams = new StreamClient(client);

  streams.on('track', (event) => {
    const [eventStream] = (event as { streams: MediaStream[] }).streams;

    if (!eventStream) {
      throw new Error('expected event stream to exist');
    }

    if (eventStream.id !== name) {
      return;
    }

    audio.srcObject = eventStream;
  });

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

<Collapse title={name}>
  <v-breadcrumbs slot="title" crumbs="audio_input" />
  <div class="h-auto border border-t-0 border-medium p-2">
    <div class="container mx-auto">
      <div class="pt-4">
        <div class="flex items-center gap-2">
          <v-switch
            id="audio-input"
            value={isOn ? 'on' : 'off'}
            on:input={toggleExpand}
          />
          <span class="pr-2">Listen</span>
        </div>

        {#if isOn}
          <audio
            class='py-2'
            controls
            autoplay
            bind:this={audio}
          />
        {/if}
      </div>
    </div>
  </div>
</Collapse>
