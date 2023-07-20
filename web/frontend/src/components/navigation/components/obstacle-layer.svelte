<script lang='ts'>

import { type Map } from 'maplibre-gl';
import { Canvas } from '@threlte/core';
import Scene from './scene.svelte';
import { cameraMatrix, mapSize } from '../stores';
import { renderPlugin } from '../plugins/render';

export let map: Map;

const canvas = map.getCanvas();

let context: WebGLRenderingContext | undefined;

const handleResize = () => {
  mapSize.update((value) => {
    value.width = canvas.clientWidth;
    value.height = canvas.clientHeight;
    return value;
  })
};

map.on('style.load', () => map.addLayer({
  id: 'obstacle-layer',
  type: 'custom',
  renderingMode: '3d',
  onAdd (_: Map, newContext: WebGLRenderingContext) {
    context = newContext;
  },
  render (_ctx, viewProjectionMatrix) {
    cameraMatrix.fromArray(viewProjectionMatrix);
    map.triggerRepaint();
  },
}));

map.on('resize', handleResize);
handleResize();
renderPlugin();

</script>

{#if context}
  <Canvas
    rendererParameters={{ canvas, context, alpha: true, antialias: true }}
    useLegacyLights={false}
    shadows={false}
    size={mapSize.current}
  >
    <Scene />
  </Canvas>
{/if}
