<script lang='ts'>

import { StreamClient, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import Collapse from '@/lib/components/collapse.svelte';
import { useRobotClient, useDisconnect } from '@/hooks/robot-client';

export let name: string;

const { robotClient } = useRobotClient();

let audio: HTMLAudioElement;

let isOn = false;

const streamClient = new StreamClient($robotClient);

const handleTrack = (event: unknown) => {
  const [eventStream] = (event as { streams: MediaStream[] }).streams;

  if (!eventStream) {
    throw new Error('expected event stream to exist');
  }

  if (eventStream.id !== name) {
    return;
  }

  audio.srcObject = eventStream;
};

const toggleExpand = async () => {
  isOn = !isOn;

  streamClient.on('track', handleTrack);

  if (isOn) {
    try {
      await streamClient.add(name);
    } catch (error) {
      displayError(error as ServiceError);
    }
    return;
  }

  try {
    await streamClient.remove(name);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

useDisconnect(() => {
  streamClient.off('track', handleTrack);
});

</script>

<Collapse title={name}>
  <v-breadcrumbs slot="title" crumbs="audio_input" />
  <div class="h-auto border border-t-0 border-medium p-2">
    <div class="flex items-center gap-2">
      <v-switch
        id="audio-input"
        label='Listen'
        value={isOn ? 'on' : 'off'}
        on:input={toggleExpand}
      />
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
</Collapse>
