<script lang='ts'>

import { Canvas } from '@threlte/core';
import Scene from './scene.svelte';
import { renderPlugin } from '../plugins/render';
import { computeBoundingPlugin } from '../plugins/compute-bounding';

renderPlugin();
computeBoundingPlugin();

</script>

<div class='absolute top-0 right-0 pointer-events-none'>
  <Canvas useLegacyLights={false}>
    <Scene />
  </Canvas>
</div>
