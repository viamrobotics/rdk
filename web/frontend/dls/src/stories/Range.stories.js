import { storiesOf } from "@storybook/vue";

storiesOf("Range", module)
  .add("Default Range", () => ({
    data() {
      return {
        percentage: 100,
        name: "Test Range:",
      };
    },
    template: '<div><Range v-model="percentage" :name="name"></Range></div>',
  }))
  .add("Angle Range", () => ({
    data() {
      return {
        angle: 90,
        name: "Angle",
      };
    },
    template:
      '<div><Range v-model="angle" :min="0" :max="360" :step="90" :name="name"></Range></div>',
  }))
  .add("Default RangeInput with degrees", () => ({
    data() {
      return {
        percentage: 50,
        name: "Test Range:",
      };
    },
    template:
      '<div><Range v-model="percentage" :name="name" unit="°"></Range></div>',
  }));
