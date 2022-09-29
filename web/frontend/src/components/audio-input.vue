<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { ref } from 'vue';
import streamApi from '../gen/proto/stream/v1/stream_pb.esm';
import { displayError } from '../lib/error';
import { normalizeRemoteName } from '../lib/resource';

interface Props {
  name: string
  crumbs: string[]
}

const props = defineProps<Props>();

const audioInput = ref(false);

const toggleExpand = () => {
  audioInput.value = !audioInput.value;
  
  const isOn = audioInput.value;

  if (isOn) {
    const req = new streamApi.AddStreamRequest();
    req.setName(props.name);
    window.streamService.addStream(req, new grpc.Metadata(), displayError);
    return;
  }

  const req = new streamApi.RemoveStreamRequest();
  req.setName(props.name);
  window.streamService.removeStream(req, new grpc.Metadata(), displayError);
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
            :id="`stream-${normalizeRemoteName(props.name)}`"
            class="clear-both h-fit transition-all duration-300 ease-in-out"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>
