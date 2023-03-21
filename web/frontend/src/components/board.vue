<script setup lang="ts">

import { toast } from '../lib/toast';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';
import { Client, BoardClient, ServiceError } from '@viamrobotics/sdk';

const props = defineProps<{
  name: string
  status: {
    analogsMap: Record<string, { value: number }>
    digitalInterruptsMap: Record<string, { value: number }>
  }
  client: Client;
}>();

const boardClient = new BoardClient(props.client, props.name, { requestLogger: rcLogConditionally });
const getPin = $ref('');
const setPin = $ref('');
const setLevel = $ref('');
const pwm = $ref('');
const pwmFrequency = $ref('');

let getPinMessage = $ref('');

const getGPIO = async () => {
  try {
    const isHigh = await boardClient.getGPIO(getPin);
    getPinMessage = `Pin: ${getPin} is ${isHigh ? 'high' : 'low'}`;
  } catch (error) {
    toast.error((error as ServiceError).message);
  }
};

const setGPIO = async () => {

  try {
    await boardClient.setGPIO(setPin, setLevel === 'high');
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const getPWM = async () => {
  try {
    const dutyCyclePct = await boardClient.getPWM(getPin);
    getPinMessage = `Pin ${getPin}'s duty cycle is ${dutyCyclePct * 100}%.`;
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const setPWM = async () => {
  try {
    await boardClient.setPWM(setPin, Number.parseFloat(pwm) / 100);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const getPWMFrequency = async () => {
  try {
    const frequencyHz = await boardClient.getPWMFrequency(getPin);
    getPinMessage = `Pin ${getPin}'s frequency is ${frequencyHz}Hz.`;
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const setPWMFrequency = async () => {
  try {
    await boardClient.setPWMFrequency(setPin, Number.parseFloat(pwmFrequency));
  } catch (error) {
    displayError(error as ServiceError);
  }
};

</script>

<template>
  <v-collapse
    :title="name"
    class="board"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="board"
    />
    <div class="overflow-auto border border-t-0 border-black p-4">
      <h3 class="mb-2">
        Analogs
      </h3>
      <table class="mb-4 table-auto border border-black">
        <tr
          v-for="(analog, analogName) in status.analogsMap"
          :key="analogName"
        >
          <th class="border border-black p-2">
            {{ analogName }}
          </th>
          <td class="border border-black p-2">
            {{ analog.value || 0 }}
          </td>
        </tr>
      </table>
      <h3 class="mb-2">
        Digital Interrupts
      </h3>
      <table class="mb-4 w-full table-auto border border-black">
        <tr
          v-for="(di, interruptName) in status.digitalInterruptsMap"
          :key="interruptName"
        >
          <th class="border border-black p-2">
            {{ interruptName }}
          </th>
          <td class="border border-black p-2">
            {{ di.value || 0 }}
          </td>
        </tr>
      </table>
      <h3 class="mb-2">
        GPIO
      </h3>
      <table class="mb-4 w-full table-auto border border-black">
        <tr>
          <th class="border border-black p-2">
            Get
          </th>
          <td class="border border-black p-2">
            <div class="flex flex-wrap items-end gap-2">
              <v-input
                label="Pin"
                type="integer"
                :value="getPin"
                @input="getPin = $event.detail.value"
              />
              <v-button
                label="Get Pin State"
                @click="getGPIO"
              />
              <v-button
                label="Get PWM"
                @click="getPWM"
              />
              <v-button
                label="Get PWM Frequency"
                @click="getPWMFrequency"
              />
              <span class="py-2">
                {{ getPinMessage }}
              </span>
            </div>
          </td>
        </tr>
        <tr>
          <th class="border border-black p-2">
            Set
          </th>
          <td class="p-2">
            <div class="flex flex-wrap items-end gap-2">
              <v-input
                v-model="setPin"
                type="integer"
                class="mr-2"
                label="Pin"
              />
              <select
                v-model="setLevel"
                class="mr-2 h-[30px] border border-black bg-white text-sm"
              >
                <option>low</option>
                <option>high</option>
              </select>
              <v-button
                class="mr-2"
                label="Set Pin State"
                @click="setGPIO"
              />
              <v-input
                v-model="pwm"
                label="PWM"
                type="number"
                class="mr-2"
              />
              <v-button
                class="mr-2"
                label="Set PWM"
                @click="setPWM"
              />
              <v-input
                v-model="pwmFrequency"
                label="PWM Frequency"
                type="number"
                class="mr-2"
              />
              <v-button
                class="mr-2"
                label="Set PWM Frequency"
                @click="setPWMFrequency"
              />
            </div>
          </td>
        </tr>
      </table>
    </div>
  </v-collapse>
</template>
