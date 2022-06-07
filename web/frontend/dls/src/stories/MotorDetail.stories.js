import { storiesOf } from "@storybook/vue";

storiesOf("MotorDetailNew", module).add("Default MotorDetailNew", () => ({
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
    '<MotorDetailNew :streamName="streamName"  :baseName="streamName" :crumbs="crumbs"  :motorStatus="motorStatus"></MotorDetailNew>',
}));