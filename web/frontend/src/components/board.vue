<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { toast } from '../lib/toast';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';
import boardApi from '../gen/component/board/v1/board_pb.esm';

interface Props {
  name: string
  status: {
    analogsMap: Record<string, { value: number }>
    digitalInterruptsMap: Record<string, { value: number }>
  }
}

const props = defineProps<Props>();

const getPin = $ref('');
const setPin = $ref('');
const setLevel = $ref('');
const pwm = $ref('');
const pwmFrequency = $ref('');

let getPinMessage = $ref('');

const getGPIO = () => {
  const req = new boardApi.GetGPIORequest();
  req.setName(props.name);
  req.setPin(getPin);

  rcLogConditionally(req);
  window.boardService.getGPIO(req, new grpc.Metadata(), (error, response) => {
    if (error) {
      toast.error(error.message);
      return;
    }

    const x = response!.toObject();

    getPinMessage = `Pin: ${getPin} is ${x.high ? 'high' : 'low'}`;
  });
};

const setGPIO = () => {
  const req = new boardApi.SetGPIORequest();
  req.setName(props.name);
  req.setPin(setPin);
  req.setHigh(setLevel === 'high');

  rcLogConditionally(req);
  window.boardService.setGPIO(req, new grpc.Metadata(), displayError);
};

const getPWM = () => {
  const req = new boardApi.PWMRequest();
  req.setName(props.name);
  req.setPin(getPin);

  rcLogConditionally(req);
  window.boardService.pWM(req, new grpc.Metadata(), (error, response) => {
    if (error) {
      toast.error(error.message);
      return;
    }
    const { dutyCyclePct } = response!.toObject();

    getPinMessage = `Pin ${getPin}'s duty cycle is ${dutyCyclePct * 100}%.`;
  });
};

const setPWM = () => {
  const req = new boardApi.SetPWMRequest();
  req.setName(props.name);
  req.setPin(setPin);
  req.setDutyCyclePct(Number.parseFloat(pwm) / 100);

  rcLogConditionally(req);
  window.boardService.setPWM(req, new grpc.Metadata(), displayError);
};

const getPWMFrequency = () => {
  const req = new boardApi.PWMFrequencyRequest();
  req.setName(props.name);
  req.setPin(getPin);

  rcLogConditionally(req);
  window.boardService.pWMFrequency(req, new grpc.Metadata(), (error, response) => {
    if (error) {
      toast.error(error.message);
      return;
    }
    const { frequencyHz } = response!.toObject();

    getPinMessage = `Pin ${getPin}'s frequency is ${frequencyHz}Hz.`;
  });
};

const setPWMFrequency = () => {
  const req = new boardApi.SetPWMFrequencyRequest();
  req.setName(props.name);
  req.setPin(setPin);
  req.setFrequencyHz(Number.parseFloat(pwmFrequency));

  rcLogConditionally(req);
  window.boardService.setPWMFrequency(req, new grpc.Metadata(), displayError);
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
            {{ name }}
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
