<script lang='ts'>

import * as THREE from 'three';
import { type Map } from 'maplibre-gl';
import { Canvas, currentWritable } from '@threlte/core'
import Scene from './scene.svelte'

const matrix4 = currentWritable(new THREE.Matrix4())

export let map: Map;

let canvas: HTMLCanvasElement | undefined;
let context: WebGLRenderingContext | undefined;

const onAdd = (map: Map, newContext: WebGLRenderingContext) => {
  canvas = map.getCanvas()
  context = newContext
};

map.on('style.load', () => map.addLayer({
  id: '3d-model',
  type: 'custom',
  renderingMode: '3d',
  onAdd,
  render (_gl, matrix) {
    matrix4.update((value) => value.fromArray(matrix))
  },
}));

</script>

{#if context}
  <Canvas
    useLegacyLights={false}
    rendererParameters={{ canvas, context }}
    frameloop='never'
  >
    <Scene {map} {matrix4} />
  </Canvas>
{/if}