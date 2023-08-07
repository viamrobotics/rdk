<script lang="ts">

import { createEventDispatcher } from 'svelte';
import { Canvas } from '@threlte/core';
import * as THREE from 'three';
import type { commonApi } from '@viamrobotics/sdk';

import Legend from './legend.svelte';
import Dropzone from '@/lib/components/dropzone.svelte';
import Scene from './scene.svelte';

export let pointcloud: Uint8Array | undefined;
export let pose: commonApi.Pose | undefined;
export let destination: THREE.Vector2 | undefined;
export let helpers: boolean;

const dispatch = createEventDispatcher();

let motionPath: string | undefined;
let basePosition = new THREE.Vector2()
let baseRotation = 0

const updatePose = (newPose: commonApi.Pose) => {
  basePosition.x = newPose.getX()
  basePosition.y = newPose.getY()
  baseRotation = THREE.MathUtils.degToRad(newPose.getTheta() - 90);
};

const handleDrop = (event: CustomEvent<string>) => {
  motionPath = event.detail;
};

$: if (pose) updatePose(pose);

</script>

<Dropzone on:drop={handleDrop}>
  <div class="relative w-full h-[400px]">
    <Canvas useLegacyLights={false}>
      <Scene
        {helpers}
        {pointcloud}
        {basePosition}
        {baseRotation}
        {destination}
        {motionPath}
        on:click={(event) => dispatch('click', event)}
      />
    </Canvas>

    {#if helpers}
      <p class="absolute left-3 top-3 bg-white text-xs">
        Grid set to 1 meter
      </p>
    {/if}

    <Legend />
  </div>
</Dropzone>
