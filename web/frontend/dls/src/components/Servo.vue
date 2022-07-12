<template>
  <div>
    <Collapse>
      <div class="flex">
          <h2 class="flex-grow p-4 text-xl">{{ servoName }}</h2>
          <div class="p-2">
          <ViamButton color="danger" group variant="primary" @click="servoStop">
            <template v-slot:icon>
              <ViamIcon color="white" :path="mdiCloseOctagonOutline"
                >STOP</ViamIcon
              >
            </template>
            STOP
          </ViamButton>
        </div>
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
              </td>
          </tr>
        </table>
      </div>
    </Collapse>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import Collapse from "./Collapse.vue";
import "vue-class-component/hooks";
import {
  mdiCloseOctagonOutline,
} from "@mdi/js";
import ViamButton from "./Button.vue";
import ViamIcon from "./ViamIcon.vue";

@Component({
  components: {
    Collapse,
    ViamButton,
    ViamIcon,
  },
})
export default class Servo extends Vue {
  @Prop({ default: null }) servoName!: string;
  @Prop({ default: null }) servoAngle!: string;

  mdiCloseOctagonOutline = mdiCloseOctagonOutline;

  servoStop(e: Event): void {
    e.preventDefault();
    e.stopPropagation();
    this.$emit("servo-stop");
  }
}
</script>
