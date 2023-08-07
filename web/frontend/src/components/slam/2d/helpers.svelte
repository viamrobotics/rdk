<script lang='ts'>

import { T, extend } from '@threlte/core'
import { AxesHelper, GridHelper } from 'trzy';
import { renderOrder } from './constants';

extend({ AxesHelper, GridHelper })

</script>

<T.GridHelper
  renderOrder={renderOrder.grid}
  color='#ddd'
  on:create={({ ref }) => ref.rotateX(Math.PI / 2)}
/>

<T.AxesHelper
  renderOrder={renderOrder.axes}
  width={0.2}
  length={10000}
/>

<T.AxesHelper
  renderOrder={renderOrder.axes}
  width={0.2}
  length={10000}
  on:create={({ ref }) => ref.rotateZ(Math.PI)}
/>
