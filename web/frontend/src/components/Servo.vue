<script setup lang="ts">

import { ref } from "vue";
import RadioButtons from "./RadioButtons.vue";
import type { Status } from "../gen/proto/api/component/motor/v1/motor_pb.esm";
import InfoButton from "./info-button.vue";

interface Props {
  motorName: string
  crumbs: [string]
  motorStatus: Status.AsObject
}

interface Emits {
  (event: 'motor-run', data: unknown): void
  (event: 'motor-stop'): void
}

defineProps<Props>()
const emit = defineEmits<Emits>()

const movementType = ref("Go");
const direction = ref<-1 | 1>(1);
const position = ref(0);
const rpm = ref(0);
const power = ref(25);
const type = ref("go");
const speed = ref(0);
const revolutions = ref(0);

const setMovementType = (input: string) => {
  movementType.value = input;
  switch (input) {
    case "Go":
      type.value = "go";
      break;
    case "Go For":
      type.value = "goFor";
      break;
    case "Go To":
      type.value = "goTo";
      break;
  }
}

const setDirection = (input: string) => {
  switch (input) {
    case "Forwards":
      direction.value = 1;
      break;
    case "Backwards":
      direction.value = -1;
      break;
    default:
      direction.value = 1;
  }
}

const motorRun = () => { 
  emit("motor-run", {
    direction: direction.value,
    position: position.value,
    rpm: rpm.value,
    power: power.value,
    type: type.value,
    speed: speed.value,
    revolutions: revolutions.value,
  });
}

const motorStop = (e: Event) => {
  emit("motor-stop");
}

</script>

<template>
  <v-collapse :title="motorName">
    <div slot="header" class="flex">
      <v-breadcrumbs :crumbs="crumbs.join(',')" />
      <div
        class="p-4 flex items-center flex-wrap"
        v-if="motorStatus.positionReporting"
      >
        <p
          class="flex items-center border border-black rounded-full px-2 leading-tight"
        >
          Position {{ motorStatus.position }}
        </p>
      </div>
      <div class="p-4 flex items-center flex-wrap">
        <v-badge color="green" label="Running" v-if="motorStatus.isPowered" />
        <v-badge color="gray" label="Idle" v-if="!motorStatus.isPowered" />
      </div>
      <v-button variant="danger" label="STOP" @click="motorStop" />
    </div>

    <div class="h-[500px]">
      <div class="h-full border border-black border-t-0 p-4 grid grid-cols-1">
        <div class="grid">
          <div class="column">
            <p class="text-xs pb-2">Set Power</p>
            <RadioButtons
              :options="
                motorStatus.positionReporting
                  ? ['Go', 'Go For', 'Go To']
                  : ['Go']
              "
              defaultOption="Go"
              :disabledOptions="[]"
              v-on:selectOption="setMovementType($event)"
            />
          </div>
          <div class="flex pt-4" v-if="movementType === 'Go To'">
            <div class="place-self-end pr-2">
              <span class="text-2xl">{{ movementType }}</span>
              <InfoButton
                class="pb-2"
                :infoRows="['Relative to Home']"
              />
            </div>
            <ViamInput
              type="number"
              color="primary"
              group="False"
              variant="primary"
              class="pr-2 w-48"
              inputId="distance"
              v-model="position"
            >
              <span class="text-xs">Position in Revolutions</span>
            </ViamInput>
            <ViamInput
              type="number"
              color="primary"
              group="False"
              variant="primary"
              class="pr-2 w-32"
              inputId="distance"
              v-model="rpm"
            >
              <span class="text-xs">RPM</span>
            </ViamInput>
          </div>
          <div class="flex pt-4" v-if="movementType === 'Go For'">
            <div class="place-self-end pr-2">
              <span class="text-2xl">{{ movementType }}</span>
              <InfoButton
                class="pb-2"
                :infoRows="['Relative to where the robot is currently']"
              />
            </div>
            <ViamInput
              type="number"
              color="primary"
              group="False"
              variant="primary"
              class="pr-2 w-32"
              inputId="distance"
              v-model="revolutions"
            >
              <span class="text-xs"># in Revolutions</span>
            </ViamInput>
            <div class="column pr-4">
              <p class="text-xs mb-1">Direction of Rotation</p>
              <RadioButtons
                :options="['Forwards', 'Backwards']"
                defaultOption="Forwards"
                :disabledOptions="[]"
                v-on:selectOption="setDirection($event)"
              />
            </div>
            <ViamInput
              type="number"
              color="primary"
              group="False"
              variant="primary"
              class="pr-2 w-32"
              inputId="distance"
              v-model="rpm"
            >
              <span class="text-xs">RPM</span>
            </ViamInput>
          </div>
          <div class="flex items-start pt-4" v-if="movementType === 'Go'">
            <div class="place-self-end pr-2">
              <span class="text-2xl">{{ movementType }}</span>
              <InfoButton
                class="pb-2"
                :infoRows="['Continuously moves']"
              />
            </div>
            <div class="column pr-4">
              <p class="text-xs pb-2 pt-1">Direction of Rotation</p>
              <RadioButtons
                :options="['Forwards', 'Backwards']"
                defaultOption="Forwards"
                :disabledOptions="[]"
                v-on:selectOption="setDirection($event)"
              />
            </div>
            <Range
              class="pt-2"
              id="power"
              :min="0"
              :max="100"
              :step="1"
              v-model="power"
              unit="%"
              name="Power %"
            />
          </div>
        </div>
        <div class="flex flex-row-reverse">
          <v-button
            @click="motorRun()"
            label="RUN"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>
