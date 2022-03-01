import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("Button", module).add("Default Button", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"]
    };
  },
  template:
    '<div><ViamButton color="primary" group variant="primary">Primary</ViamButton><ViamButton color="success" group variant="primary">Success</ViamButton><ViamButton color="danger" group variant="primary">Danger</ViamButton></div>',
}));
