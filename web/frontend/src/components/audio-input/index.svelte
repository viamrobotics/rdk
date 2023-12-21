<script lang='ts'>

import { StreamClient, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import Collapse from '@/lib/components/collapse.svelte';
import { useRobotClient } from '@/hooks/robot-client';

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

const toggle = async (value: boolean) => {
  isOn = value;

  if (isOn) {
    try {
      streamClient.on('track', handleTrack);
      await streamClient.add(name);
    } catch (error) {
      displayError(error as ServiceError);
    }
    return;
  }

  try {
    streamClient.off('track', handleTrack);
    await streamClient.remove(name);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

</script>

<Collapse title={name}>
  <v-breadcrumbs slot="title" crumbs="audio_input" />
  <div class="h-auto border border-t-0 border-medium p-2">
    <audio
      bind:this={audio}
      class='py-2'
      controls
      on:play={async () => toggle(true)}
      on:pause={async () => toggle(false)}
    />
  </div>
</Collapse>
