<script setup lang="ts">

import { onMounted } from 'vue';
import type { ServiceError } from '../gen/proto/stream/v1/stream_pb_service.esm';
import { displayError } from '../lib/error';
import { addStream, removeStream } from '../lib/stream';
import { type StreamServiceClient, createStreamService } from '../api';

interface Props {
  name: string
  crumbs: string[]
}

const props = defineProps<Props>();

let streamService: StreamServiceClient;

let isOn = $ref(false);

const toggleExpand = async () => {
  isOn = !isOn;

  if (isOn) {
    try {
      await addStream(props.name, streamService);
    } catch (error) {
      displayError(error as ServiceError);
    }
    return;
  }

  try {
    await removeStream(props.name, streamService);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

onMounted(() => {
  streamService = createStreamService();
});

</script>

<template>
  <v-collapse :title="name">
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
              :value="isOn ? 'on' : 'off'"
              @input="toggleExpand"
            />
            <span class="pr-2">Listen</span>
          </div>

          <div
            v-if="isOn"
            :data-stream="props.name"
            class="clear-both h-fit transition-all duration-300 ease-in-out"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>
