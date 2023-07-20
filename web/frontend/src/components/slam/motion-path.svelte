<script lang='ts'>

import * as THREE from 'three';
import { Line2 } from 'three/examples/jsm/lines/Line2.js';
import { LineMaterial } from 'three/examples/jsm/lines/LineMaterial.js';
import { LineGeometry } from 'three/examples/jsm/lines/LineGeometry.js';

export let scene: THREE.Scene;
export let path: string | undefined;

const geometry = new LineGeometry();
const material = new LineMaterial({
  color: 0xFF_00_47,
  linewidth: 0.005,
  dashed: false,
  alphaToCoverage: true,
});

const line = new Line2(geometry, material);
line.visible = false;
scene.add(line);

const updatePath = (pathstr?: string) => {
  if (pathstr === undefined) {
    line.visible = false
    return
  }

  line.visible = true

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
}

$: updatePath(path)

</script>
