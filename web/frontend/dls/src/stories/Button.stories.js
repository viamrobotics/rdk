import { storiesOf } from "@storybook/vue";

storiesOf("Button", module).add("Default Button", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"],
    };
  },
  template:
    '<div><ViamButton color="primary" group variant="primary"><template v-slot:icon><ViamIcon  name="check">Check</ViamIcon></template>Primary</ViamButton><ViamButton color="black" group variant="primary"><template v-slot:icon><ViamIcon  name="check">Check</ViamIcon></template>Primary</ViamButton><ViamButton color="success" group variant="primary">Success</ViamButton><ViamButton color="danger" group variant="primary">Danger</ViamButton></div>',
}));
