<div class="flex">
    <h2 class="p-4 text-xl">{{ servoName }}</h2>
</div>
<template v-slot:content>
<div class="flex border border-black border-t-0 p-4">
    <table  class="table-auto border-collapse border border-black">
    <tr>
        <td class="border border-black p-2">Angle</td>
        <td class="border border-black p-2">{{ servoAngle }}</td>
    </tr>
    <tr>
        <td class="border border-black p-2"></td>
        <td class="border border-black p-2">
          <ViamButton group @click="$emit('servo-minus-ten')">-10</ViamButton>
          <ViamButton group @click="$emit('servo-minus-one')">-1</ViamButton>
          <ViamButton group @click="$emit('servo-plus-one')">1</ViamButton>
          <ViamButton group @click="$emit('servo-plus-ten')">10</ViamButton>
           @click="$emit('base-change-tab', selectedItem)"
        </td>
    </tr>
    </table>
</div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import "vue-class-component/hooks";
import ViamButton from "./Button.vue";

@Component({
  components: {
    ViamButton,
  },
})
export default class Servo extends Vue {
  @Prop({ default: null }) servoName!: string;
  @Prop({ default: null }) servoAngle!: string;
}
</script>
