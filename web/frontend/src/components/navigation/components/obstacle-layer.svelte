<script lang='ts'>

import { type Map } from 'maplibre-gl';
import { Canvas } from '@threlte/core';
import Scene from './scene.svelte';
import { mapCamera, mapCameraViewProjectionMatrix } from '../stores';
import { injectLngLatPlugin } from '../lnglat-plugin';

export let map: Map;

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
    mapCamera.projectionMatrix.fromArray(nextViewProjectionMatrix)
    mapCameraViewProjectionMatrix.set(nextViewProjectionMatrix);
  },
}));

map.on('resize', handleResize);

</script>

{#if context}
  <Canvas
    rendererParameters={{ canvas, context, alpha: true, antialias: true }}
    useLegacyLights={false}
    shadows={false}
    size={{ width, height }}
    frameloop='always'
  >
    <Scene {map} />
  </Canvas>
{/if}
