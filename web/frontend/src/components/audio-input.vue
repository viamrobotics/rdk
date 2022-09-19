<script setup lang="ts">

import { ref } from 'vue';
import { normalizeRemoteName } from '../lib/resource';

interface Props {
  streamName: string
  crumbs: string[]
}

interface Emits {
  (event: 'toggle-input', isOn: boolean): void
}

const props = defineProps<Props>();

const emit = defineEmits<Emits>();

const audioInput = ref(false);

const toggleExpand = () => {
  audioInput.value = !audioInput.value;
  emit('toggle-input', audioInput.value);
};

</script>

<template>
  <v-collapse :title="streamName">
    <v-breadcrumbs
      slot="title"
      :crumbs="crumbs.join(',')"
    />
    <div class="h-auto border-x border-b border-black p-2">
      <div class="container mx-auto">
        <div class="pt-4">
          <div class="flex items-center gap-2">
            <v-switch
              id="audio-input"
              :value="audioInput ? 'on' : 'off'"
              @input="toggleExpand()"
            />
            <span class="pr-2">Listen</span>
          </div>

          <div
            v-if="audioInput"
            :id="`stream-${normalizeRemoteName(props.streamName)}`"
            class="clear-both h-fit transition-all duration-300 ease-in-out"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>
