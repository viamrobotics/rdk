<template>
  <div id="app">
    <MotorDetail
      motorName="MOTOR NAME"
      :motorStatus="{
        on: false,
        positionSupported: true,
        position: 0,
        pidConfig: {
          fieldsMap: [
            ['Kd', { numberValue: 0 }],
            ['Kp', { numberValue: 0.34 }],
            ['Ki', { numberValue: 0.77 }],
          ],
        },
      }"
    />
  </div>
</template>

<script lang="ts">
import { Component, Vue } from "vue-property-decorator";
import MotorDetail from "./components/MotorDetail.vue";

@Component({
  components: {
    MotorDetail,
  },
})
export default class App extends Vue {}
</script>

<style>
@import "./assets/css/styles.css";
@import "./assets/css/fontawesome.min.css";
@import "./assets/css/solid.min.css";
@import "./assets/css/regular.min.css";
#app {
  width: 1080px;
}
</style>
