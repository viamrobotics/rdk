<script lang='ts'>

import { StreamClient, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { useRobotClient, useConnect } from '@/hooks/robot-client';

export let name: string;

const { robotClient } = useRobotClient();

let audio: HTMLAudioElement;

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

useConnect(() => {
  streamClient.on('track', handleTrack);
  streamClient.add(name).catch((error) => displayError(error as ServiceError))

  return () => {
    streamClient.remove(name).catch((error) => displayError(error as ServiceError))
    streamClient.off('track', handleTrack);
  }
})

</script>

<div class="h-auto border border-t-0 border-medium p-2">
  <audio
    class='py-2'
    controls
    bind:this={audio}
  />
</div>
