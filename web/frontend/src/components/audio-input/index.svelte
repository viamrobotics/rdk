<script lang='ts'>

import { StreamClient, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import Collapse from '@/lib/components/collapse.svelte';
import { useConnect, useRobotClient } from '@/hooks/robot-client';
import { onDestroy, onMount } from 'svelte';

export let name: string;

const { robotClient } = useRobotClient();

let audio: HTMLAudioElement;
let expanded = false;
let connected = false;
let added = false;

const streamClient = new StreamClient($robotClient);

const handleTrack = (event: unknown) => {
  const [eventStream] = (event as { streams: MediaStream[] }).streams;

  if (!eventStream) {
    throw new Error('expected event stream to exist');
  }

  if (eventStream.id !== name) {
    return;
  }

  console.log(audio, eventStream)
  audio.srcObject = eventStream;
};

const handleToggle = (event: CustomEvent<{ open: boolean }>) => {
  expanded = event.detail.open;
};

useConnect(() => {
  connected = true;

  return () => {
    connected = false;
  }
});


onMount(() => {
  streamClient.on('track', handleTrack);
  return () => streamClient.off('track', handleTrack)
});

$: if (connected && expanded && !added) {
  streamClient.add(name).catch((error) => displayError(error as ServiceError));
  added = true;
} else if (added) {
  streamClient.remove(name).catch((error) => displayError(error as ServiceError));
  added = false;
}

</script>

<Collapse title={name} on:toggle={handleToggle}>
  <v-breadcrumbs slot="title" crumbs="audio_input" />
  <div class="h-auto border border-t-0 border-medium p-2">
    <audio
      bind:this={audio}
      class='py-2'
      controls
      volume={1}
    />
  </div>
</Collapse>
