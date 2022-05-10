import { storiesOf } from "@storybook/vue";

storiesOf("NumberInput", module)
  .add("NumberInput with floats", () => ({
    data() {
      return {
        value: 12.3,
      };
    },
    template: `<div><number-input v-model="value" :float="true"></number-input></div>`,
  }))
  .add("NumberInput without floats but with min=5 and max=1000", () => ({
    data() {
      return {
        value: 7,
      };
    },
    template: `<div><number-input v-model="value" :min="5" :max="1000"></number-input></div>`,
  }))
  .add("NumberInput readonly", () => ({
    data() {
      return {
        value: 7,
      };
    },
    template: `<div><number-input v-model="value" :readonly="true"></number-input></div>`,
  }));
