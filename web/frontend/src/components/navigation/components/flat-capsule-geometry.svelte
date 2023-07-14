<script lang='ts'>

import * as THREE from 'three'
import { T } from '@threlte/core'
import { mergeGeometries } from 'three/examples/jsm/utils/BufferGeometryUtils'

export let args: [radius?: number | undefined, length?: number | undefined, segments?: number] = []

let geometry: THREE.BufferGeometry

$: {
  const [radius = 1, length = 2, segments = 32] = args

  geometry = mergeGeometries([
    // Top circle
    new THREE.CircleGeometry(radius, segments).translate(0, length / 2, 0).rotateX(Math.PI / 4),
    // Bottom circle
    // new THREE.CircleGeometry(radius, segments).translate(0, -length / 2, 0).rotateX(Math.PI),
    // Rectangular body
    // new THREE.PlaneGeometry(length, radius)
    // new THREE.CylinderGeometry(radius, radius, length, segments),
  ]);

  geometry = new THREE.CircleGeometry(radius, segments)
    .translate(0, length / 2, 0)
    .rotateX(-Math.PI / 2)
}

</script>

<T is={geometry} />
