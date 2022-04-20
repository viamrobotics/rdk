import { storiesOf } from "@storybook/vue";

storiesOf("NumberInput", module)
.add("NumberInput with floats", () => ({
  data() {
    return {
      value: 12.3,
    };
  },
  template: `<div><number-input v-model="value" float></number-input></div>`,
}))
.add("NumberInput without floats and min/max", () => ({
  data() {
    return {
      value: 7,
    };
  },
  template: `<div><number-input v-model="value" :min="5" :max="10"></number-input></div>`,
}));
