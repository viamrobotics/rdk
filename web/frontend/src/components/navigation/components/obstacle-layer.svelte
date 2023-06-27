<script lang='ts'>

import { type Map } from 'maplibre-gl';
import { Canvas, currentWritable } from '@threlte/core';
import Scene from './scene.svelte';
import { injectLngLatPlugin } from '../lnglat-plugin';
import type { Mat4 } from '../types';

export let map: Map;

const viewProjectionMatrix = currentWritable<Float32Array | Mat4>(null!);
const canvas = map.getCanvas();

let context: WebGLRenderingContext | undefined;
let width = 0;
let height = 0;

const handleResize = () => {
  width = canvas.clientWidth;
  height = canvas.clientHeight;
};

injectLngLatPlugin();
handleResize();

map.on('style.load', () => map.addLayer({
  id: 'obstacle-layer',
  type: 'custom',
  renderingMode: '3d',
  onAdd (_: Map, newContext: WebGLRenderingContext) {
    context = newContext;
  },
  render (_ctx, nextViewProjectionMatrix) {
    viewProjectionMatrix.set(nextViewProjectionMatrix);
  },
}));

map.on('resize', handleResize);

</script>

{#if context}
  <Canvas
    rendererParameters={{ canvas, context }}
    useLegacyLights={false}
    shadows={false}
    size={{ width, height }}
    frameloop='always'
  >
    <Scene {map} {viewProjectionMatrix} />
  </Canvas>
{/if}
