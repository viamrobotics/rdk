<script setup lang="ts">

import { computed, ref } from 'vue';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import { Client, commonApi, genericApi } from '@viamrobotics/sdk';
import { toast } from '../lib/toast';
import { resourceNameToString } from '../lib/resource';
import { rcLogConditionally } from '../lib/log';

interface Props {
  resources: commonApi.ResourceName.AsObject[];
  client: Client
}

const props = defineProps<Props>();

const resources = computed(() => props.resources);

const selectedComponent = ref();
const input = ref();
const output = ref();
const executing = ref(false);

const doCommand = (name: string, command: string) => {
  if (!name || !command) {
    return;
  }
  const request = new genericApi.DoCommandRequest();
  request.setName(name);
  request.setCommand(Struct.fromJavaScript(JSON.parse(command)));

  executing.value = true;
  rcLogConditionally(request);
  props.client.genericService.doCommand(request, (error, response) => {
    if (error) {
      toast.error(`Error executing command on ${name}: ${error}`);
      executing.value = false;
      return;
    }

    if (!response) {
      toast.error(`Invalid response when executing command on ${name}`);
      executing.value = false;
      return;
    }

    output.value = JSON.stringify(response?.getResult()?.toObject(), null, '\t');
    executing.value = false;
  });
};

const namesToPrettySelect = (resourcesToPretty: commonApi.ResourceName.AsObject[]): string => {
  const simple = new Map<string, number>();

  for (const resource of resourcesToPretty) {
    if (!simple.has(resource.name)) {
      simple.set(resource.name, 0);
    }
    simple.set(resource.name, simple.get(resource.name)! + 1);
  }

  return resourcesToPretty.map((res) => {
    if (simple.get(res.name) === 1) {
      return res.name;
    }
    return resourceNameToString(res);
  }).join(',');
};

</script>

<template>
  <v-collapse
    title="DoCommand()"
    class="doCommand"
  >
    <div class="h-full w-full border border-t-0 border-black p-4">
      <v-select
        label="Selected Component"
        placeholder="Select a component"
        :options="namesToPrettySelect(resources)"
        :value="selectedComponent"
        :disabled="executing ? 'true' : 'false'"
        class="mb-4"
        @input="selectedComponent = $event.detail.value"
      />
      <div class="flex flex-wrap h-full w-full flex-row gap-2">
        <div class="h-full w-full">
          <p class="text-large">
            Input
          </p>
          <div class="h-[250px] w-full max-w-full border border-black p-2">
            <v-code-editor
              language="json"
              value="{}"
              @input="input = $event.detail.value"
            />
          </div>
        </div>
        <div class="flex min-w-[90px] flex-col justify-center">
          <v-button
            variant="inverse-primary"
            :label="executing ? 'RUNNING...' : 'DO'"
            :disabled="!selectedComponent || !input || executing ? 'true' : 'false'"
            @click="doCommand(selectedComponent, input)"
          />
        </div>
        <div class="h-full w-full">
          <p class="text-large">
            Output
          </p>
          <div class="h-[250px] w-full border border-black p-2">
            <v-code-editor
              language="json"
              :value="output"
              readonly="true"
            />
          </div>
        </div>
      </div>
    </div>
  </v-collapse>
</template>
