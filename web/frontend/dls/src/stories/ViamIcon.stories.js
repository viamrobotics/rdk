import { storiesOf } from "@storybook/vue";
import { mdiRotateLeft } from "@mdi/js";

storiesOf("ViamIcon", module).add("Default ViamIcon", () => ({
  data() {
    return {
      mdiRotateLeft,
    };
  },
  template: '<div><ViamIcon title="test" :path="mdiRotateLeft" /></div>',
}));
