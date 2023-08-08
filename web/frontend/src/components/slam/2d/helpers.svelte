<!--
  @component
  Helpers for orienting the user spacially within a map.
  Contains a grid and axes, with small grid spaces placed at 1m.
-->
<script lang='ts'>

import { T, extend } from '@threlte/core';
import { AxesHelper, GridHelper } from 'trzy';
import { renderOrder } from './constants';

extend({
  // trzy.AxesHelper uses thick lines so that axes are more clearly displayed, and is dynamically resizable.
  AxesHelper,
  // trzy.GridHelper is more performant by drawing the grid in a shader, and can have small and large cells.
  GridHelper,
});

</script>

<T.GridHelper
  renderOrder={renderOrder.grid}
  color='#ddd'
  on:create={({ ref }) => ref.rotateX(Math.PI / 2)}
/>

<T.AxesHelper
  renderOrder={renderOrder.axes}
  width={0.2}
  length={10_000}
/>

<T.AxesHelper
  renderOrder={renderOrder.axes}
  width={0.2}
  length={10_000}
  on:create={({ ref }) => ref.rotateZ(Math.PI)}
/>
