<script lang="ts">

import { ArmClient, Client } from '@viamrobotics/sdk';
import type { Pose, ServiceError } from '@viamrobotics/sdk';
import { copyToClipboard } from '@/lib/copy-to-clipboard';
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

type Field = 'x' | 'y' | 'z' | 'oX' | 'oY' | 'oZ' | 'theta'

export let name: string;
export let status: {
  is_moving: boolean
  end_position: Record<string, number>
  joint_positions: { values: number[]}
} | undefined;
export let client: Client;

let modifyAll = false;

const fieldSetters = [
  ['x', 'X'],
  ['y', 'Y'],
  ['z', 'Z'],
  ['theta', 'Theta'],
  ['o_x', 'OX'],
  ['o_y', 'OY'],
  ['o_z', 'OZ'],
] as const;

$: posPieces = fieldSetters.map((setter) => {
  const [endPositionField] = setter;
  return {
    endPosition: setter,
    endPositionValue: status?.end_position[endPositionField!] || 0,
  };
});

/*
 * this conditional is in place so the RC card renders when
 * the fake arm is not using any kinematics file
 */
$: jointPieces = status?.joint_positions.values.map((value, index) => {
  return {
    joint: index,
    jointValue: value ?? 0,
  };
}) ?? [{ joint: 0, jointValue: 100 }];

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

let modifyAllStatus: ArmStatus = {
  pos_pieces: [],
  joint_pieces: [],
}

const armClient = new ArmClient(client, name, { requestLogger: rcLogConditionally });

const stop = async () => {
  try {
    await armClient.stop();
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const armModifyAllDoEndPosition = async () => {
  const newPieces = modifyAllStatus.pos_pieces;

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

  modifyAll = false
};

const armModifyAllCancel = () => {
  modifyAll = false
};

const armModifyAllDoJoint = async () => {
  const arm = status!;
  const newList = arm.joint_positions.values;
  const newPieces = modifyAllStatus.joint_pieces;

  for (let i = 0; i < newPieces.length && i < newList.length; i += 1) {
    newList[newPieces[i]!.joint] = newPieces[i]!.jointValue;
  }

  try {
    await armClient.moveToJointPositions(newList);
  } catch (error) {
    displayError(error as ServiceError);
  }
  
  modifyAll = false
};

const armEndPositionInc = async (updateField: string | undefined, amount: number) => {
  if (updateField === undefined) {
    return;
  }

  const adjustedAmount = updateField[0] === 'o' || updateField[0] === 'O' ? amount / 100 : amount;
  const arm = status!;
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
  const arm = status!;
  const newList = arm.joint_positions.values;
  newList[field] += amount;

  try {
    await armClient.moveToJointPositions(newList);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const armHome = async () => {
  const arm = status!;
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
  const nextPos = []
  const nextJoint = []

  for (const posPiece of posPieces) {
    nextPos.push({
      endPosition: [...posPiece!.endPosition],
      endPositionValue: roundTo2Decimals(posPiece!.endPositionValue),
    });
  }

  for (const jointPiece of jointPieces) {
    nextJoint.push({
      joint: jointPiece!.joint,
      jointValue: roundTo2Decimals(jointPiece!.jointValue),
    });
  }

  modifyAllStatus = {
    pos_pieces: nextPos,
    joint_pieces: nextJoint,
  }
  modifyAll = true
};

const armCopyPosition = () => {
  // eslint-disable-next-line unicorn/no-array-reduce
  copyToClipboard(JSON.stringify(posPieces.reduce((acc, cur) => {
    return {
      ...acc,
      [`${cur.endPosition[0]}`]: cur.endPositionValue,
    };
  }, {})));
};

const armCopyJoints = () => {
  // eslint-disable-next-line unicorn/no-array-reduce
  copyToClipboard(JSON.stringify(jointPieces.reduce((acc, cur) => {
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

  <v-button
    slot="header"
    class="flex items-center justify-between gap-2"
    variant="danger"
    icon="stop-circle"
    label="Stop"
    on:click|stopPropagation={stop}
  />

  <div class="border border-t-0 border-medium p-4 text-sm">
    {#if status}
      <div class="flex flex-wrap gap-12">
        <div>
          <h3 class="mb-2 font-bold flex items-center gap-2">
            End position (mms)
            <v-button
              variant='icon'
              icon='copy'
              on:click={armCopyPosition}
            />
          </h3>

          <div class="flex flex-col gap-1 pb-1">
            {#if modifyAll}
              {#each modifyAllStatus.pos_pieces as piece (piece.endPosition[0])}
                <label class="py-1 pr-2 text-right">{piece.endPosition[1]}</label>
                <input
                  bind:value={piece.endPositionValue}
                  class="border border-medium px-4 py-1"
                />
              {/each}
            {:else}
              {#each posPieces as piece (piece.endPosition[0])}
                <div class='flex gap-1'>
                  <h4 class='self-center justify-self-end min-w-[3rem] text-right pr-2'>
                    {piece.endPosition[1]}
                  </h4>
                  <v-button
                    class='place-self-center'
                    label="--"
                    on:click={() => armEndPositionInc(piece.endPosition[1], -10)}
                  />
                  <v-button
                    class='place-self-center'
                    label="-"
                    on:click={() => armEndPositionInc(piece.endPosition[1], -1)}
                  />
                  {#if modifyAll}
                    <input
                      type='number'
                      value={piece.endPositionValue.toFixed(2)}
                      class="w-[5rem] py-1.5 px-2 leading-tight text-xs h-[30px] border outline-none appearance-none pl-2.5 bg-white border-light hover:border-medium focus:border-gray-9"
                    />
                  {:else}
                    <p class='place-self-center min-w-[5rem] text-xs flex place-content-center'>
                      {piece.endPositionValue.toFixed(2)}
                    </p>
                  {/if}
                  <v-button
                    class='place-self-center'
                    label="+"
                    on:click={() => armEndPositionInc(piece.endPosition[1], 1)}
                  />
                  <v-button
                    class='place-self-center'
                    label="++"
                    on:click={() => armEndPositionInc(piece.endPosition[1], 10)}
                  />
                </div>
              {/each}
            {/if}
          </div>

          <div class='mt-6'>
            <v-button
              label="Home"
              on:click={armHome}
            />
            {#if modifyAll}
              <v-button
                class="whitespace-nowrap"
                label="Cancel"
                on:click={() => { modifyAll = false }}
              />
              <v-button
                class="mr-4 whitespace-nowrap"
                label="Go To End Position"
                on:click={armModifyAllDoEndPosition}
              />
              <v-button
                label="Go To Joints"
                on:click={armModifyAllDoJoint}
              />
            {:else}
              <v-button
                class="whitespace-nowrap"
                label="Modify all"
                on:click={armModifyAll}
              />
            {/if}
          </div>
        </div>
  
        <div>
          <h3 class="mb-2 font-bold flex items-center gap-2">
            Joints (degrees)
            <v-button
              variant='icon'
              icon='copy'
              on:click={armCopyJoints}
            />
          </h3>

          <div class="flex flex-col gap-1 pb-1">
            {#if modifyAll}
              {#each modifyAllStatus.joint_pieces ?? [] as piece (piece.joint)}
                <label class="py-1 pr-2 text-right">Joint {piece.joint}</label>
                <input
                  bind:value={piece.jointValue}
                  class="border border-medium px-4 py-1"
                >
              {/each}
            {:else}
              {#each jointPieces as piece (piece.joint)}
                <div class='flex gap-1'>
                  <h4 class="self-center justify-self-end min-w-[4rem] text-right pr-2">
                    Joint {piece.joint}
                  </h4>
                  <v-button
                    class='place-self-center'
                    label="--"
                    on:click={() => armJointInc(piece.joint, -10)}
                  />
                  <v-button
                    class='place-self-center'
                    label="-"
                    on:click={() => armJointInc(piece.joint, -1)}
                  />
                  {#if modifyAll}
                    <input
                      type='number'
                      value={piece.jointValue.toFixed(2)}
                      class="w-[5rem] py-1.5 px-2 leading-tight text-xs h-[30px] border outline-none appearance-none pl-2.5 bg-white border-light hover:border-medium focus:border-gray-9"
                    />
                  {:else}
                    <p class='place-self-center min-w-[5rem] text-xs flex place-content-center'>
                      {piece.jointValue.toFixed(2)}
                    </p>
                  {/if}
                  <v-button
                    class='place-self-center'
                    label="+"
                    on:click={() => armJointInc(piece.joint, 1)}
                  />
                  <v-button
                    class='place-self-center'
                    label="++"
                    on:click={() => armJointInc(piece.joint, 10)}
                  />
                </div>
              {/each}
            {/if}
          </div>
        </div>
      </div>
    {/if}
  </div>
</Collapse>
