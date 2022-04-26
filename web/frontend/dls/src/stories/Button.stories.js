import { storiesOf } from "@storybook/vue";
import { mdiRestore, mdiReload } from "@mdi/js";

storiesOf("Button", module).add("Default Button", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"],
      mdiRestore,
      mdiReload,
    };
  },
  template:
    '<div class="flex"><ViamButton color="primary" group variant="primary"><template v-slot:icon><ViamIcon  :path="mdiRestore">Check</ViamIcon></template>Primary</ViamButton><ViamButton color="black" group variant="primary"><template v-slot:icon><ViamIcon color="white"  :path="mdiReload">Check</ViamIcon></template>Primary</ViamButton><ViamButton color="success" group variant="primary">Success</ViamButton><ViamButton color="danger" group variant="primary">Danger</ViamButton><ViamButton color="success" group variant="primary" loading="true">Loading</ViamButton></div>',
}));
