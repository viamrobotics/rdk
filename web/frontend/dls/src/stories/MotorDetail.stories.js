import { storiesOf } from "@storybook/vue";

storiesOf("MotorDetail", module).add("Default MotorDetail", () => ({
  data() {
    return {
      streamName: "Left Motor",
      crumbs: ["Motor"],
      motorStatus: {
        isOn: true,
        positionReporting: true,
        position: 10,
      },
    };
  },
  template:
    '<MotorDetail :streamName="streamName"  :baseName="streamName" :crumbs="crumbs"  :motorStatus="motorStatus"></MotorDetail>',
}));