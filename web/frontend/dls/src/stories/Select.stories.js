import { storiesOf } from "@storybook/vue";

storiesOf("Select", module)
  .add("Default Select", () => ({
    data() {
      return {
        value: 1,
        options: [
            { label: 'no camera', id: 1},
            { label: 'camera', id: 2},
        ]
      };
    },
    template: '<div><Select v-model="value" :options="options" value-key="id"></Select></div>',
  }));
