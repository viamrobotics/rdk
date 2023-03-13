<script setup lang="ts">

import { ref } from 'vue';
import { StreamClient } from '@viamrobotics/sdk';
import type { Client, ServiceError } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';

const props = defineProps<{
  name: string
  client: Client
}>();

const audioInput = ref(false);

const toggleExpand = async () => {
  audioInput.value = !audioInput.value;

  const isOn = audioInput.value;

  const streams = new StreamClient(props.client);
  if (isOn) {
    try {
      await streams.add(props.name);
    } catch (error) {
      displayError(error as ServiceError);
    }
    return;
  }

  try {
    await streams.remove(props.name);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

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
              :value="audioInput ? 'on' : 'off'"
              @input="toggleExpand"
            />
            <span class="pr-2">Listen</span>
          </div>

          <div
            v-if="audioInput"
            :data-stream="props.name"
            class="clear-both h-fit transition-all duration-300 ease-in-out"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>
