<script setup lang="ts">

import { computed } from 'vue';
import { Status } from "../gen/proto/api/component/inputcontroller/v1/input_controller_pb.esm";

interface Props {
  controllerName: string
  controllerStatus: Status.AsObject
}

const props = defineProps<Props>()

const controlOrder = [
  "AbsoluteX",
  "AbsoluteY",
  "AbsoluteRX",
  "AbsoluteRY",
  "AbsoluteZ",
  "AbsoluteRZ",
  "AbsoluteHat0X",
  "AbsoluteHat0Y",
  "ButtonSouth",
  "ButtonEast",
  "ButtonWest",
  "ButtonNorth",
  "ButtonLT",
  "ButtonRT",
  "ButtonLThumb",
  "ButtonRThumb",
  "ButtonSelect",
  "ButtonStart",
  "ButtonMenu",
  "ButtonEStop",
];

const connected = computed(() => {
  for (let { event } of props.controllerStatus.eventsList) {
    if (event != "Disconnect") {
      return true;
    }
  }
  return false;
})

const getValue = (controlMatch: string) => {
  for (const { control, value } of props.controllerStatus.eventsList) {
    if (control === controlMatch) {
      if (control.includes("Absolute")) {
        return value.toFixed(4);
      } else {
        return value.toFixed(0);
      }
    }
  }

  return "";
}

const controls: string[][] = [];

for (const ctrl of controlOrder) {
  const value = getValue(ctrl);
  if (value != "") {
    controls.push([
      ctrl.replace("Absolute", "").replace("Button", ""),
      value,
    ]);
  }
}

</script>

<template>
  <v-collapse :title="`${controllerName} Input`">
    <div slot="header" class="p-4 flex items-center flex-wrap">
      <ViamBadge color="green" v-if="connected">Connected</ViamBadge>
      <ViamBadge color="gray" v-if="!connected">Disconnected</ViamBadge>
    </div>
    <div class="border border-black border-t-0 p-4">
      <template v-if="connected">
        <div
          v-for="control in controls"
          :key="control[0]"
          class="column control"
        >
          <p class="subtitle">{{ control[0] }}</p>
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

.header {
  display: flex;
  flex-direction: row;
  align-items: center;
  align-content: center;
  gap: 8px;
}

.row {
  display: flex;
  flex-direction: row;
  margin-right: 12px;
  gap: 8px;
  margin-bottom: 12px;
}

.subtitle {
  color: var(--black-70);
}

.column {
  display: flex;
  flex-direction: column;
  margin-left: 0px;
}

.control {
  width: 8ex;
}

.margin-bottom {
  margin-bottom: 32px;
}
</style>
