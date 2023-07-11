<script lang='ts'>

import * as THREE from 'three'
import { Line2 } from 'three/examples/jsm/lines/Line2.js'
import { LineMaterial } from 'three/examples/jsm/lines/LineMaterial.js'
import { LineGeometry } from 'three/examples/jsm/lines/LineGeometry.js'

export let scene: THREE.Scene;
export let path: string | undefined;

const geometry = new LineGeometry();
const material = new LineMaterial({
  color: 0xFF0047,
  linewidth: 0.005,
  dashed: false,
  alphaToCoverage: true,
});

const line = new Line2(geometry, material);
line.visible = false,
scene.add(line);

$: {
  if (path === undefined) {
    line.visible = false
  } else {
    line.visible = true

    let points: number[] = []

    for (const line of path.split('\n')) {
      const [a, b] = line.split(',')
      if (a !== undefined && b !== undefined) {
        const x = Number.parseFloat(a!) / 1000
        const y = Number.parseFloat(b!) / 1000
        points.push(x, y, 0)
      }
    }

    const vertices = new Float32Array(points)
    geometry.setPositions(vertices);
  
  }
}

</script>
