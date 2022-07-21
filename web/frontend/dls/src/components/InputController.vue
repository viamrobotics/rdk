<template>
  <div class="pb-8">
    <Collapse>
      <div class="flex">
        <h2 class="p-4 text-xl">{{ controllerName }} Input</h2>
        <div class="p-4 flex items-center flex-wrap">
          <ViamBadge color="green" v-if="connected">Connected</ViamBadge>
          <ViamBadge color="gray" v-if="!connected">Disconnected</ViamBadge>
        </div>
      </div>
      <template v-slot:content>
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
      </template>
    </Collapse>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import { Status } from "proto/api/component/inputcontroller/v1/input_controller_pb";

@Component
export default class InputController extends Vue {
  @Prop() controllerName!: string;
  @Prop() controllerStatus!: Status.AsObject;

  self = this;

  get controls(): string[][] {
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
    var controls = [];
    for (const ctrl of controlOrder) {
      var value = this.getValue(ctrl);
      if (value != "") {
        controls.push([
          ctrl.replace("Absolute", "").replace("Button", ""),
          value,
        ]);
      }
    }
    return controls;
  }

  get connected(): boolean {
    for (let ev of this.controllerStatus.eventsList) {
      if (ev.event != "Disconnect") {
        return true;
      }
    }
    return false;
  }

  getValue(control: string): string {
    for (const iEvent of this.controllerStatus.eventsList) {
      if (iEvent.control === control) {
        if (control.includes("Absolute")) {
          return iEvent.value.toFixed(4);
        } else {
          return iEvent.value.toFixed(0);
        }
      }
    }
    return "";
  }
}
</script>

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
