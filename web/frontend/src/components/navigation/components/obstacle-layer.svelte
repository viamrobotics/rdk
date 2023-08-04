<script lang='ts'>

import { onDestroy, onMount } from 'svelte';
import { Canvas } from '@threlte/core';
import { type Map } from 'maplibre-gl';
import Scene from './scene.svelte';
import { cameraMatrix, mapSize } from '../stores';
import { renderPlugin } from '../plugins/render';

export let map: Map;

const handleResize = () => {
  mapSize.update((value) => {
    const canvas = map.getCanvas();
    value.width = canvas.clientWidth;
    value.height = canvas.clientHeight;
    return value;
  });
};

const addLayer = () => map.addLayer({
  id: 'obstacle-layer',
  type: 'custom',
  renderingMode: '3d',
  render (_ctx, viewProjectionMatrix) {
    cameraMatrix.fromArray(viewProjectionMatrix);
    map.triggerRepaint();
  },
});

renderPlugin();

onMount(() => {
  map.on('resize', handleResize);
  handleResize();

  if (map.isStyleLoaded()) {
    addLayer()
  } else {
    map.on('style.load', () => addLayer())
  }
})

onDestroy(() => {
  map.off('resize', handleResize)

  if (map.getLayer('obstacle-layer')) {
    map.removeLayer('obstacle-layer')
  }
})

</script>

<div class='absolute top-0 right-0 pointer-events-none'>
  <Canvas useLegacyLights={false}>
    <Scene />
  </Canvas>
</div>
