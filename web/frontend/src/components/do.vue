<script setup lang="ts">

import { computed, ref } from 'vue';
import { Struct } from "google-protobuf/google/protobuf/struct_pb";
import type { GenericServiceClient } from '../gen/proto/api/component/generic/v1/generic_pb_service.esm';
import type { Resource } from '../lib/resource';
import genericApi from '../gen/proto/api/component/generic/v1/generic_pb.esm';

interface Props {
  resources: Resource[]
}

const props = defineProps<Props>();

const resources = computed({ get: () => props.resources, set: () => ({}) });
const selectedComponent = ref();

const value = JSON.stringify({
  foo: 'bar',
  some: 'thing',
  yes: true,
  count: 10,
});
</script>

<script lang="ts">
export default {
  data() {
    return {
      testJson: JSON.stringify({
        foo: 'bar',
        some: 'thing',
        yes: true,
        count: 10,
      })
    }
  },
  methods: {
    doCommand: (name: string, command: any) => {
      console.log('do', { name, command });
      const request = new genericApi.DoRequest();

      const x = Struct.create(command);
      console.log('do', { request, x });

      request.setName(name);
      request.setCommand(command);

      console.log('do', { request });

      ((window as any).genericService as GenericServiceClient).do(request, (error, response) => {
        if (error) {
          console.error(`Error executing command on ${name}`, error);
        }

        if (!response) {
          console.error(`Invalid response when executing command on ${name}`, response);
        }

        console.log('response', response);
      });
    },
  },
};
</script>

<template>
  <v-collapse :title="`Do()`">
    <div class="border border-t-0 border-black p-4">
      {{ props.resources.map(({ name }) => name).join(',') }}
      <v-select
        label="Selected Component"
        placeholder="Null"
        :options="resources.map(({ name }) => name).join()"
        :value="selectedComponent"
        @input="selectedComponent = $event.detail.value"
      />
      <div class="flex flex-row">
        <div>
          <p class="text-large">
            Input
          </p>
          <div class="h-[200px] w-full border border-black p-2">
            <v-code-editor
              language="json"
              :value="testJson"
            />
          </div>
        </div>
        <button 
          @click="doCommand(selectedComponent, JSON.parse(value))"
        >
          Do
        </button>
      </div>
    </div>
  </v-collapse>
</template>

<style scoped>
</style>
