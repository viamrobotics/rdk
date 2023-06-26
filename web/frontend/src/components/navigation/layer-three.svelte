<script lang='ts'>

import { type Map } from 'maplibre-gl';
import { Canvas, currentWritable } from '@threlte/core'
import Scene from './scene.svelte'
import { injectLngLatPlugin } from './lnglat-plugin'

injectLngLatPlugin();

export let map: Map;

const viewProjectionMatrix = currentWritable<Float32Array | [number, number, number, number, number, number, number, number, number, number, number, number, number, number, number, number]>(null!);

let canvas: HTMLCanvasElement | undefined;
let context: WebGLRenderingContext | undefined;

let width = map.getCanvas().clientWidth
let height = map.getCanvas().clientHeight

const onAdd = (map: Map, newContext: WebGLRenderingContext) => {
  canvas = map.getCanvas()
  context = newContext
};

map.on('style.load', () => map.addLayer({
  id: 'obstacle-layer',
  type: 'custom',
  renderingMode: '3d',
  onAdd,
  render (_ctx, nextViewProjectionMatrix) {
    viewProjectionMatrix.set(nextViewProjectionMatrix)
  },
}));

map.on('resize', () => {
  width = map.getCanvas().clientWidth
  height = map.getCanvas().clientHeight
})

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
