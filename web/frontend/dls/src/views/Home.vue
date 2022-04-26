<template>
  <div class="home">
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
import { Component, Vue } from 'vue-property-decorator';
import InputController from '@/components/InputController.vue';
import MotorDetail from '@/components/MotorDetail.vue';
import WebGamepad from '@/components/WebGamepad.vue'; // @ is an alias to /src

@Component({
  components: {
    InputController,
    MotorDetail,
    WebGamepad,
  },
})
export default class Home extends Vue {}
</script>
