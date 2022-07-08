<script setup lang="ts">

import { computed } from 'vue';
import type { Status } from '../gen/proto/api/component/inputcontroller/v1/input_controller_pb.esm';

interface Props {
  controllerName: string
  controllerStatus: Status.AsObject
}

const props = defineProps<Props>();

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
  for (const { event } of props.controllerStatus.eventsList) {
    if (event !== 'Disconnect') {
      return true;
    }
  }
  return false;
});

const getValue = (controlMatch: string) => {
  for (const { control, value } of props.controllerStatus.eventsList) {
    if (control === controlMatch) {
      return control.includes('Absolute') ? value.toFixed(4) : value.toFixed(0);
    }
  }

  return '';
};

const controls: string[][] = [];

for (const ctrl of controlOrder) {
  const value = getValue(ctrl);
  if (value !== '') {
    controls.push([
      ctrl.replace('Absolute', '').replace('Button', ''),
      value,
    ]);
  }
}

</script>

<template>
  <v-collapse :title="`${controllerName} Input`">
    <div
      slot="header"
      class="flex flex-wrap items-center p-4"
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
          class="column control"
        >
          <p class="subtitle">
            {{ control[0] }}
          </p>
          {{ control[1] }}
        </div>
      </template>
    </div>
  </v-collapse>
</template>

<style scoped>
p,
h2,
h3 {
  margin: 0;
}

.subtitle {
  color: var(--black-70);
}

.column {
  display: flex;
  flex-direction: column;
  margin-left: 0px;
}

</style>
