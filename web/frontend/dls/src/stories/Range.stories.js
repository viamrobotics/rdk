import { storiesOf } from "@storybook/vue";

storiesOf("Range", module).add("Default Range", () => ({
  data() {
    return {
      percentage: 50,
      name: "Test Range:",
    };
  },
  template: '<div><Range :percentage=percentage :name="name"></Range></div>',
}));
