<template>
  <div class="component">
    <Container>
      <div  class="flex flex-row border">
        <div class="row flex-auto w-5/6 basis-1/2 p-1">
          <div class="header flex p-1">
            <h2>{{ controllerName }}</h2>
            <Breadcrumbs :crumbs="controls"></Breadcrumbs>
          </div>
        </div>

        <div class="row basis-1/4 p-1">
          <div
            v-for="control in controls"
            :key="control[0]"
            class="column control"
          >
            <p class="subtitle">{{ control[0] }}</p>
            {{ control[1] }}
          </div>
          <div class="flex">
            <ViamSwitch class="p-3" name="Test" size="sm" id="test" :option=connected></ViamSwitch>
            <div class="p-2">
              <ViamBadge v-if="connected" color="green">Connected</ViamBadge>
              <ViamBadge v-else color="red">Disconnected</ViamBadge>
            </div>
          </div>
        </div>
      </div>
    </Container>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";

import { InputControllerStatus } from "proto/api/v1/robot_pb";
import ViamBadge from "./Badge.vue";
import ViamSwitch from "./Switch.vue";
import Container from "./Container.vue";
import Breadcrumbs from "./Breadcrumbs.vue";

@Component({
  components: {
    Breadcrumbs,
    Container,
    ViamSwitch,
    ViamBadge
  },
})
export default class InputController extends Vue {
  @Prop() controllerName!: string;
  @Prop() controllerStatus!: InputControllerStatus.AsObject;
  @Prop() option!: Boolean;

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

</style>
