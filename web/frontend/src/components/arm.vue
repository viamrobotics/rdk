<script setup lang="ts">
import { ArmClient, Client } from '@viamrobotics/sdk';
import type { Pose, ServiceError } from '@viamrobotics/sdk';
import { copyToClipboardWithToast } from '../lib/copy-to-clipboard';
import { displayError } from '../lib/error';
import { roundTo2Decimals } from '../lib/math';
import { rcLogConditionally } from '../lib/log';

export interface ArmStatus {
  pos_pieces: {
    endPosition: string[]
    endPositionValue: number
  }[]

  joint_pieces: {
    joint: number
    jointValue: number
  }[]
}

export interface RawArmStatus extends ArmStatus {
  joint_positions: {
    values: number[]
  }
  end_position: Record<string, number>
}

type Field = 'x' | 'y' | 'z' | 'oX' | 'oY' | 'oZ' | 'theta'

const props = defineProps<{
  name: string
  status?: ArmStatus,
  rawStatus?: RawArmStatus
  client: Client
}>();

const fieldMap = [
  ['x', 'x'],
  ['y', 'y'],
  ['z', 'z'],
  ['theta', 'theta'],
  ['o_x', 'oX'],
  ['o_y', 'oY'],
  ['o_z', 'oZ'],
] as const;

const updateFieldMap: Record<string, Field> = {
  X: 'x',
  Y: 'y',
  Z: 'z',
  Theta: 'theta',
  OX: 'oX',
  OY: 'oY',
  OZ: 'oZ',
} as const;

const toggle = $ref<Record<string, ArmStatus>>({});

const armClient = new ArmClient(props.client, props.name, { requestLogger: rcLogConditionally });

const stop = async () => {
  try {
    await armClient.stop();
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const armModifyAllDoEndPosition = async () => {
  const newPieces = toggle[props.name]!.pos_pieces;

  const newPose: Pose = {
    x: 0,
    y: 0,
    z: 0,
    oX: 0,
    oY: 0,
    oZ: 0,
    theta: 0,
  };

  for (const newPiece of newPieces) {
    const [, poseField] = newPiece.endPosition;
    const field: Field = updateFieldMap[poseField!]!;
    newPose[field] = Number(newPiece.endPositionValue);
  }

  try {
    await armClient.moveToPosition(newPose);
  } catch (error) {
    displayError(error as ServiceError);
  }
  delete toggle[props.name];
};

const armModifyAllCancel = () => {
  delete toggle[props.name];
};

const armModifyAllDoJoint = async () => {
  const arm = props.rawStatus!;
  const newList = arm.joint_positions.values;
  const newPieces = toggle[props.name]!.joint_pieces;

  for (let i = 0; i < newPieces.length && i < newList.length; i += 1) {
    newList[newPieces[i]!.joint] = newPieces[i]!.jointValue;
  }

  try {
    await armClient.moveToJointPositions(newList);
  } catch (error) {
    displayError(error as ServiceError);
  }
  delete toggle[props.name];
};

const armEndPositionInc = async (updateField: string, amount: number) => {
  const adjustedAmount = updateField[0] === 'o' || updateField[0] === 'O' ? amount / 100 : amount;
  const arm = props.rawStatus!;
  const old = arm.end_position;

  const newPose: Pose = {
    x: 0,
    y: 0,
    z: 0,
    oX: 0,
    oY: 0,
    oZ: 0,
    theta: 0,
  };

  for (const [endPositionField, poseField] of fieldMap) {
    const endPositionValue = old[endPositionField] || 0;
    const field: Field = poseField;
    newPose[field] = Number(endPositionValue);
  }

  const field: Field = updateFieldMap[updateField]!;
  newPose[field] += adjustedAmount;

  try {
    await armClient.moveToPosition(newPose);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const armJointInc = async (field: number, amount: number) => {
  const arm = props.rawStatus!;
  const newList = arm.joint_positions.values;
  newList[field] += amount;

  try {
    await armClient.moveToJointPositions(newList);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const armHome = async () => {
  const arm = props.rawStatus!;
  const newList = arm.joint_positions.values;

  for (let i = 0; i < newList.length; i += 1) {
    newList[i] = 0;
  }

  try {
    await armClient.moveToJointPositions(newList);
  } catch (error) {
    displayError(error as ServiceError);
  }
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

const armCopyPosition = (status: ArmStatus) => {
  // eslint-disable-next-line unicorn/no-array-reduce
  copyToClipboardWithToast(JSON.stringify(status.pos_pieces.reduce((acc, cur) => {
    return {
      ...acc,
      [`${cur.endPosition[0]}`]: cur.endPositionValue,
    };
  }, {})));
};

const armCopyJoints = (status: ArmStatus) => {
  // eslint-disable-next-line unicorn/no-array-reduce
  copyToClipboardWithToast(JSON.stringify(status.joint_pieces.reduce((acc, cur) => {
    return {
      ...acc,
      [`${cur.joint}`]: cur.jointValue,
    };
  }, {})));
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
    <div class="border-border-1 border border-t-0 p-4">
      <div class="mb-4 flex flex-wrap gap-4">
        <div
          v-if="toggle[name]"
          class="border-border-1 border p-4"
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
                class="border-border-1 border px-4 py-1"
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
          class="border-border-1 border p-4"
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
                class="border-border-1 border px-4 py-1"
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
          class="border-border-1 border p-4"
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
            <v-button
              label="Copy"
              class="flex-auto text-right"
              @click="() => armCopyPosition(status!)"
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
          class="border-border-1 border p-4"
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
            <v-button
              label="Copy"
              class="flex-auto text-right"
              @click="() => armCopyJoints(status!)"
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
