import { storiesOf } from "@storybook/vue";

storiesOf("Range", module)
.add("Default Range", () => ({
  data() {
    return {
      percentage: 50,
      name: "Test Range:",
    };
  },
  template: '<div><Range v-model="percentage" :name="name"></Range></div>',
}))
.add("Default RangeInput with degrees", () => ({
  data() {
    return {
      percentage: 50,
      name: "Test Range:",
    };
  },
  template: '<div><Range v-model="percentage" :name="name" unit="°"></Range></div>',
}));
