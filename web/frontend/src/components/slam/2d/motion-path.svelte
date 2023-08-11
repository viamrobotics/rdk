<!--
  @component
  Renders a motion plan using thick lines.
  Assumes the motion plan is coming in as a string in the format:
  x,y\n
  x,y\n
  ...
-->
<script lang='ts'>

import { T, extend } from '@threlte/core';
import { Line2 } from 'three/examples/jsm/lines/Line2.js';
import { LineMaterial } from 'three/examples/jsm/lines/LineMaterial.js';
import { LineGeometry } from 'three/examples/jsm/lines/LineGeometry.js';
import { renderOrder } from './constants';

extend({ Line2, LineMaterial });

export let path: string | undefined;

let geometry: LineGeometry = new LineGeometry();

const updatePath = (pathstr?: string) => {
  if (pathstr === undefined) {
    return;
  }

  geometry = new LineGeometry();

  const points: number[] = [];

  for (const xy of pathstr.split('\n')) {
    const [xString, yString] = xy.split(',');
    if (xString !== undefined && yString !== undefined) {
      const x = Number.parseFloat(xString) / 1000;
      const y = Number.parseFloat(yString) / 1000;
      points.push(x, y, 0);
    }
  }

  const vertices = new Float32Array(points);
  geometry.setPositions(vertices);
};

$: updatePath(path);

</script>

{#if path}
  <T.Line2 renderOrder={renderOrder.motionPath}>
    <T.LineMaterial
      color='#FF0047'
      linewidth={0.005}
    />
    <T is={geometry} />
  </T.Line2>
{/if}
