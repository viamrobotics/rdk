import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("Select", module).add("Default Select", () => ({
  data() {
    return {
      name: "Test",
      placeholder: "Select item",
      value: ["test1", "test2"]
    };
  },
  template:
    '<div><Select><template v-slot:label>Test</template><option>Test1</option><option>Test2</option><template v-slot:error>Error</template></Select></div>',
}));
