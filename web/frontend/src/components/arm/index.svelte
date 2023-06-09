<script lang="ts">

import { ArmClient, Client } from '@viamrobotics/sdk';
import type { Pose, ServiceError } from '@viamrobotics/sdk';
import { copyToClipboardWithToast } from '@/lib/copy-to-clipboard';
import { displayError } from '@/lib/error';
import { roundTo2Decimals } from '@/lib/math';
import { rcLogConditionally } from '@/lib/log';
import Collapse from '@/components/collapse.svelte';

interface ArmStatus {
  pos_pieces: {
    endPosition: string[]
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

type Field = 'x' | 'y' | 'z' | 'oX' | 'oY' | 'oZ' | 'theta'

export let name: string;
export let status: ArmStatus | undefined;
export let rawStatus: RawArmStatus | undefined;
export let client: Client;

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

const toggle: Record<string, ArmStatus> = {};

const armClient = new ArmClient(client, name, { requestLogger: rcLogConditionally });

const stop = async () => {
  try {
    await armClient.stop();
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const armModifyAllDoEndPosition = async () => {
  const newPieces = toggle[name]!.pos_pieces;

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
  delete toggle[name];
};

const armModifyAllCancel = () => {
  delete toggle[name];
};

const armModifyAllDoJoint = async () => {
  const arm = rawStatus!;
  const newList = arm.joint_positions.values;
  const newPieces = toggle[name]!.joint_pieces;

  for (let i = 0; i < newPieces.length && i < newList.length; i += 1) {
    newList[newPieces[i]!.joint] = newPieces[i]!.jointValue;
  }

  try {
    await armClient.moveToJointPositions(newList);
  } catch (error) {
    displayError(error as ServiceError);
  }
  delete toggle[name];
};

const armEndPositionInc = async (updateField: string | undefined, amount: number) => {
  if (updateField === undefined) {
    return;
  }

  const adjustedAmount = updateField[0] === 'o' || updateField[0] === 'O' ? amount / 100 : amount;
  const arm = rawStatus!;
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
  const arm = rawStatus!;
  const newList = arm.joint_positions.values;
  newList[field] += amount;

  try {
    await armClient.moveToJointPositions(newList);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const armHome = async () => {
  const arm = rawStatus!;
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
  const arm = status!;
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

  toggle[name] = newStatus;
};

const armCopyPosition = () => {
  // eslint-disable-next-line unicorn/no-array-reduce
  copyToClipboardWithToast(JSON.stringify(status?.pos_pieces.reduce((acc, cur) => {
    return {
      ...acc,
      [`${cur.endPosition[0]}`]: cur.endPositionValue,
    };
  }, {})));
};

const armCopyJoints = () => {
  // eslint-disable-next-line unicorn/no-array-reduce
  copyToClipboardWithToast(JSON.stringify(status?.joint_pieces.reduce((acc, cur) => {
    return {
      ...acc,
      [`${cur.joint}`]: cur.jointValue,
    };
  }, {})));
};

</script>

<Collapse title={name}>
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
      on:click|stopPropagation={stop}
    />
  </div>
  <div class="border border-t-0 border-medium p-4">
    <div class="mb-4 flex flex-wrap gap-4">
      {#if toggle[name]}
        <div class="border border-medium p-4">
          <h3 class="mb-2">
            END POSITION (mms)
          </h3>

          <div class="inline-grid grid-cols-2 gap-1 pb-1">
            {#each (toggle[name]?.pos_pieces ?? []) as piece (piece.endPosition[0])}
              <label class="py-1 pr-2 text-right">{piece.endPosition[1]}</label>
              <input
                bind:value={piece.endPositionValue}
                class="border border-medium px-4 py-1"
              />
            {/each}
          </div>

          <div class="mt-2 flex gap-2">
            <v-button
              class="mr-4 whitespace-nowrap"
              label="Go To End Position"
              on:click={armModifyAllDoEndPosition}
            />
            <div class="flex-auto text-right">
              <v-button
                label="Cancel"
                on:click={armModifyAllCancel}
              />
            </div>
          </div>
        </div>
        <div class="border border-medium p-4">
          <h3 class="mb-2">
            JOINTS (degrees)
          </h3>
          <div class="grid grid-cols-2 gap-1 pb-1">
            {#each (toggle[name]?.joint_pieces ?? []) as piece (piece.joint)}
              <label class="py-1 pr-2 text-right">Joint {piece.joint}</label>
              <input
                bind:value={piece.jointValue}
                class="border border-medium px-4 py-1"
              >
            {/each}
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              label="Go To Joints"
              on:click={armModifyAllDoJoint}
            />
            <div class="flex-auto text-right">
              <v-button
                label="Cancel"
                on:click={armModifyAllCancel}
              />
            </div>
          </div>
        </div>
      {/if}
    </div>

    <div class="flex flex-wrap gap-4">
      {#if status}
        <div class="border border-medium p-4">
          <h3 class="mb-2">
            END POSITION (mms)
          </h3>
          <div class="inline-grid grid-cols-6 gap-1 pb-1">
            {#each status.pos_pieces as piece (piece.endPosition[0])}
              <h4 class="py-1 pr-2 text-right">{piece.endPosition[1]}</h4>
              <v-button
                label="--"
                on:click={() => armEndPositionInc(piece.endPosition[1], -10)}
              />
              <v-button
                label="-"
                on:click={() => armEndPositionInc(piece.endPosition[1], -1)}
              />
              <v-button
                label="+"
                on:click={() => armEndPositionInc(piece.endPosition[1], 1)}
              />
              <v-button
                label="++"
                on:click={() => armEndPositionInc(piece.endPosition[1], 10)}
              />
              <h4 class="py-1">
                {piece.endPositionValue.toFixed(2)}
              </h4>
            {/each}
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              label="Home"
              on:click={armHome}
            />
            <v-button
              label="Copy"
              class="flex-auto text-right"
              on:click={armCopyPosition}
            />
            <div class="flex-auto text-right">
              <v-button
                class="whitespace-nowrap"
                label="Modify All"
                on:click={armModifyAll}
              />
            </div>
          </div>
        </div>
        <div class="border border-medium p-4">
          <h3 class="mb-2">
            JOINTS (degrees)
          </h3>
          <div class="inline-grid grid-cols-6 gap-1 pb-1">
            {#each status.joint_pieces as piece (piece.joint)}
              <h4 class="whitespace-nowrap py-1 pr-2 text-right">
                Joint {piece.joint}
              </h4>
              <v-button
                label="--"
                on:click={() => armJointInc(piece.joint, -10)}
              />
              <v-button
                label="-"
                on:click={() => armJointInc(piece.joint, -1)}
              />
              <v-button
                label="+"
                on:click={() => armJointInc(piece.joint, 1)}
              />
              <v-button
                label="++"
                on:click={() => armJointInc(piece.joint, 10)}
              />
              <h4 class="py-1 pl-2">
                {piece.jointValue.toFixed(2)}
              </h4>
            {/each}
          </div>
          <div class="mt-2 flex gap-2">
            <v-button
              label="Home"
              on:click={armHome}
            />
            <v-button
              label="Copy"
              class="flex-auto text-right"
              on:click={armCopyJoints}
            />
            <div class="flex-auto text-right">
              <v-button
                class="whitespace-nowrap"
                label="Modify All"
                on:click={armModifyAll}
              />
            </div>
          </div>
        </div>
      {/if}
    </div>
  </div>
</Collapse>
