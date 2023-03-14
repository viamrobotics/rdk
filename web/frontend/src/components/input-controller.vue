<script setup lang="ts">

import { computed } from 'vue';
import type { inputControllerApi } from '@viamrobotics/sdk';

const props = defineProps<{
  name: string
  status: inputControllerApi.Status.AsObject
}>();

const controlOrder = [
  'AbsoluteX',
  'AbsoluteY',
  'AbsoluteRX',
  'AbsoluteRY',
  'AbsoluteZ',
  'AbsoluteRZ',
  'AbsoluteHat0X',
  'AbsoluteHat0Y',
  'ButtonSouth',
  'ButtonEast',
  'ButtonWest',
  'ButtonNorth',
  'ButtonLT',
  'ButtonRT',
  'ButtonLThumb',
  'ButtonRThumb',
  'ButtonSelect',
  'ButtonStart',
  'ButtonMenu',
  'ButtonEStop',
];

const connected = computed(() => {
  for (const { event } of props.status.eventsList) {
    if (event !== 'Disconnect') {
      return true;
    }
  }
  return false;
});

const getValue = (controlMatch: string) => {
  for (const { control, value } of props.status.eventsList) {
    if (control === controlMatch) {
      return control.includes('Absolute') ? value.toFixed(4) : value.toFixed(0);
    }
  }

  return '';
};

const controls = $computed(() => {
  const pendingControls = [];

  for (const ctrl of controlOrder) {
    const value = getValue(ctrl);
    if (value !== '') {
      pendingControls.push([
        ctrl.replace('Absolute', '').replace('Button', ''),
        value,
      ]);
    }
  }

  return pendingControls;
});

</script>

<template>
  <v-collapse :title="`${name}`">
    <v-breadcrumbs
      slot="title"
      crumbs="input_controller"
    />
    <div
      slot="header"
      class="flex flex-wrap items-center"
    >
      <v-badge
        v-if="connected"
        color="green"
        label="Connected"
      />
      <v-badge
        v-if="!connected"
        color="gray"
        label="Disconnected"
      />
    </div>
    <div class="border border-t-0 border-black p-4">
      <template v-if="connected">
        <div
          v-for="control in controls"
          :key="control[0]"
          class="ml-0 flex w-[8ex] flex-col"
        >
          <p class="subtitle m-0">
            {{ control[0] }}
          </p>
          {{ control[1] }}
        </div>
      </template>
    </div>
  </v-collapse>
</template>

<style scoped>

.subtitle {
  color: var(--black-70);
}

</style>
