<!--
  @component
  A 2D marker that can accept a texture.
  Used for rendering the base position, endpoints, etc.
-->
<script lang='ts' context='module'>

import * as THREE from 'three';

const textureLoader = new THREE.TextureLoader();

</script>

<script lang='ts'>

import { T } from '@threlte/core';
import { renderOrder } from './constants';

export let name: string;
export let url: string;
export let rotation = 0;

</script>

<T.Sprite
  {name}
  {...$$restProps}
  renderOrder={renderOrder.markers}
>
  <T.SpriteMaterial
    map={textureLoader.load(url)}
    sizeAttenuation={false}
    color='#FF0047'
    {rotation}
  />
</T.Sprite>

