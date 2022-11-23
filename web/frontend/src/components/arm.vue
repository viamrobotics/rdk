<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { Client, armApi, commonApi } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { roundTo2Decimals } from '../lib/math';

interface ArmStatus {
  pos_pieces: {
    endPosition: string
    endPositionValue: number
  }[]

  joint_pieces: {
    joint: number
    jointValue: number
  }[]
}

interface RawArmStatus extends ArmStatus {
  joint_positions: {
    values: number[]
  }
  end_position: Record<string, number>
}

interface Props {
  name: string
  status?: ArmStatus,
  rawStatus?: RawArmStatus
  client: Client
}

type GetterKeys = 'getX' | 'getY' | 'getZ' | 'getOX' | 'getOY' | 'getOZ' | 'getTheta'
type SetterKeys = 'setX' | 'setY' | 'setZ' | 'setOX' | 'setOY' | 'setOZ' | 'setTheta'

const fieldSetters = [
  ['x', 'X'],
  ['y', 'Y'],
  ['z', 'Z'],
  ['theta', 'Theta'],
  ['o_x', 'OX'],
  ['o_y', 'OY'],
  ['o_z', 'OZ'],
] as const;

const props = defineProps<Props>();

const toggle = $ref<Record<string, ArmStatus>>({});

const stop = () => {
  const request = new armApi.StopRequest();
  request.setName(props.name);
  props.client.armService.stop(request, new grpc.Metadata(), displayError);
};

const armModifyAllDoEndPosition = () => {
  const newPose = new commonApi.Pose();
  const newPieces = toggle[props.name]!.pos_pieces;

  for (const newPiece of newPieces) {
    const [, getterSetter] = newPiece.endPosition;
    const setter = `set${getterSetter}` as SetterKeys;
    newPose[setter](newPiece.endPositionValue);
  }

  const req = new armApi.MoveToPositionRequest();
  req.setName(props.name);
  req.setTo(newPose);
  props.client.armService.moveToPosition(req, new grpc.Metadata(), displayError);

  delete toggle[props.name];
};

const armModifyAllCancel = () => {
  delete toggle[props.name];
};

const armModifyAllDoJoint = () => {
  const arm = props.rawStatus!;
  const newPositionDegs = new armApi.JointPositions();
  const newList = arm.joint_positions.values;
  const newPieces = toggle[props.name]!.joint_pieces;

  for (let i = 0; i < newPieces.length && i < newList.length; i += 1) {
    newList[newPieces[i]!.joint] = newPieces[i]!.jointValue;
  }

  newPositionDegs.setValuesList(newList);

  const req = new armApi.MoveToJointPositionsRequest();
  req.setName(props.name);
  req.setPositions(newPositionDegs);
  props.client.armService.moveToJointPositions(req, new grpc.Metadata(), displayError);
  delete toggle[props.name];
};

const armEndPositionInc = (getterSetter: string, amount: number) => {
  const adjustedAmount = getterSetter[0] === 'o' || getterSetter[0] === 'O' ? amount / 100 : amount;
  const arm = props.rawStatus!;
  const old = arm.end_position;
  const newPose = new commonApi.Pose();

  for (const fieldSetter of fieldSetters) {
    const [endPositionField] = fieldSetter;
    const endPositionValue = old[endPositionField] || 0;
    const setter = `set${fieldSetter[1]}` as SetterKeys;
    newPose[setter](endPositionValue);
  }

  const getter = `get${getterSetter}` as GetterKeys;
  const setter = `set${getterSetter}` as SetterKeys;
  newPose[setter](newPose[getter]() + adjustedAmount);
  const req = new armApi.MoveToPositionRequest();
  req.setName(props.name);
  req.setTo(newPose);
  props.client.armService.moveToPosition(req, new grpc.Metadata(), displayError);
};

const armJointInc = (field: number, amount: number) => {
  const arm = props.rawStatus!;
  const newPositionDegs = new armApi.JointPositions();
  const newList = arm.joint_positions.values;
  newList[field] += amount;
  newPositionDegs.setValuesList(newList);

  const req = new armApi.MoveToJointPositionsRequest();
  req.setName(props.name);
  req.setPositions(newPositionDegs);
  props.client.armService.moveToJointPositions(req, new grpc.Metadata(), displayError);
};

const armHome = () => {
  const arm = props.rawStatus!;
  const newPositionDegs = new armApi.JointPositions();
  const newList = arm.joint_positions.values;

  for (let i = 0; i < newList.length; i += 1) {
    newList[i] = 0;
  }

  newPositionDegs.setValuesList(newList);

  const req = new armApi.MoveToJointPositionsRequest();
  req.setName(props.name);
  req.setPositions(newPositionDegs);
  props.client.armService.moveToJointPositions(req, new grpc.Metadata(), displayError);
};

const armModifyAll = () => {
  const arm = props.status!;
  const newStatus: ArmStatus = {
    pos_pieces: [],
    joint_pieces: [],
  };

  for (let i = 0; i < arm.pos_pieces.length; i += 1) {
    newStatus.pos_pieces.push({
      endPosition: arm.pos_pieces[i]!.endPosition,
      endPositionValue: roundTo2Decimals(arm.pos_pieces[i]!.endPositionValue),
    });
  }

  for (let i = 0; i < arm.joint_pieces.length; i += 1) {
    newStatus.joint_pieces.push({
      joint: arm.joint_pieces[i]!.joint,
      jointValue: roundTo2Decimals(arm.joint_pieces[i]!.jointValue),
    });
  }

  toggle[props.name] = newStatus;
};

</script>

<template>
  <v-collapse
    :title="name"
    class="arm"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="arm"
    />
    <div
      slot="header"
      class="flex items-center justify-between gap-2"
    >
      <v-button
        variant="danger"
        icon="stop-circle"
        label="STOP"
        @click.stop="stop()"
      />
    </div>
    <div class="border border-t-0 border-black p-4">
      <div class="flex flex-wrap gap-4 mb-4">
        <div
          v-if="toggle[name]"
          class="border border-black p-4"
        >
          <h3 class="mb-2">
            END POSITION (mms)
          </h3>
          <div class="inline-grid grid-cols-2 gap-1 pb-1">
            <template
              v-for="cc in toggle[name]!.pos_pieces"
              :key="cc.endPosition[0]"
            >
              <label class="py-1 pr-2 text-right">{{ cc.endPosition[1] }}</label>
              <input
                v-model="cc.endPositionValue"
                class="border border-black py-1 px-4"
              >
            </template>
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              class="mr-4 whitespace-nowrap"
              label="Go To End Position"
              @click="armModifyAllDoEndPosition"
            />
            <div class="flex-auto text-right">
              <v-button
                label="Cancel"
                @click="armModifyAllCancel"
              />
            </div>
          </div>
        </div>
        <div
          v-if="toggle[name]"
          class="border border-black p-4"
        >
          <h3 class="mb-2">
            JOINTS (degrees)
          </h3>
          <div class="grid grid-cols-2 gap-1 pb-1">
            <template
              v-for="bb in toggle[name]!.joint_pieces"
              :key="bb.joint"
            >
              <label class="py-1 pr-2 text-right">Joint {{ bb.joint }}</label>
              <input
                v-model="bb.jointValue"
                class="border border-black py-1 px-4"
              >
            </template>
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              label="Go To Joints"
              @click="armModifyAllDoJoint"
            />
            <div class="flex-auto text-right">
              <v-button
                label="Cancel"
                @click="armModifyAllCancel"
              />
            </div>
          </div>
        </div>
      </div>

      <div class="flex flex-wrap gap-4">
        <div
          v-if="status"
          class="border border-black p-4"
        >
          <h3 class="mb-2">
            END POSITION (mms)
          </h3>
          <div class="inline-grid grid-cols-6 gap-1 pb-1">
            <template
              v-for="aa in status.pos_pieces"
              :key="aa.endPosition[0]"
            >
              <h4 class="py-1 pr-2 text-right">
                {{ aa.endPosition[1] }}
              </h4>
              <v-button
                label="--"
                @click="armEndPositionInc(aa.endPosition[1]!, -10)"
              />
              <v-button
                label="-"
                @click="armEndPositionInc(aa.endPosition[1]!, -1)"
              />
              <v-button
                label="+"
                @click="armEndPositionInc(aa.endPosition[1]!, 1)"
              />
              <v-button
                label="++"
                @click="armEndPositionInc(aa.endPosition[1]!, 10)"
              />
              <h4 class="py-1">
                {{ aa.endPositionValue.toFixed(2) }}
              </h4>
            </template>
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              label="Home"
              @click="armHome"
            />
            <div class="flex-auto text-right">
              <v-button
                class="whitespace-nowrap"
                label="Modify All"
                @click="armModifyAll"
              />
            </div>
          </div>
        </div>
        <div
          v-if="status"
          class="border border-black p-4"
        >
          <h3 class="mb-2">
            JOINTS (degrees)
          </h3>
          <div class="inline-grid grid-cols-6 gap-1 pb-1">
            <template
              v-for="aa in status.joint_pieces"
              :key="aa.joint"
            >
              <h4 class="whitespace-nowrap py-1 pr-2 text-right">
                Joint {{ aa.joint }}
              </h4>
              <v-button
                label="--"
                @click="armJointInc(aa.joint, -10)"
              />
              <v-button
                label="-"
                @click="armJointInc(aa.joint, -1)"
              />
              <v-button
                label="+"
                @click="armJointInc(aa.joint, 1)"
              />
              <v-button
                label="++"
                @click="armJointInc(aa.joint, 10)"
              />
              <h4 class="py-1 pl-2">
                {{ aa.jointValue.toFixed(2) }}
              </h4>
            </template>
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              label="Home"
              @click="armHome"
            />
            <div class="flex-auto text-right">
              <v-button
                class="whitespace-nowrap"
                label="Modify All"
                @click="armModifyAll"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  </v-collapse>
</template>
