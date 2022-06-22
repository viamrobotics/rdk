import Vue, { VueConstructor } from "vue";
import "./assets/css/styles.css";
import "./assets/css/fontawesome.min.css";
import "./assets/css/solid.min.css";
import "./assets/css/regular.min.css";
import "./index.css";

import MotorDetail from "./components/MotorDetail.vue";
import InputController from "./components/InputController.vue";
import WebGamepad from "./components/WebGamepad.vue";
import ViamBadge from "./components/Badge.vue";
import Breadcrumbs from "./components/Breadcrumbs.vue";
import ViamButton from "./components/Button.vue";
import Collapse from "./components/Collapse.vue";
import Container from "./components/Container.vue";
import Grid from "./components/Grid.vue";
import Range from "./components/Range.vue";
import RadioButtons from "./components/RadioButtons.vue";
import ViamSwitch from "./components/Switch.vue";
import Tab from "./components/Tab.vue";
import Tabs from "./components/Tabs.vue";
import ViamIcon from "./components/ViamIcon.vue";
import ViamInput from "./components/ViamInput.vue";
import ViamBase from "./components/Base.vue";
import KeyboardInput from "./components/KeyboardInput.vue";
import Camera from "./components/Camera.vue";
import ViamSelect from "./components/ViamSelect.vue";
import ViamInfoButton from "./components/ViamInfoButton.vue";
import Popper from "vue-popperjs";
import NumberInput from "./components/NumberInput.vue";
import Slam from "./components/Slam.vue";
import ClickOutside from "./directives/clickOutside";

const Components: { [key: string]: VueConstructor<Vue> } = {
  MotorDetail,
  InputController,
  WebGamepad,
  Collapse,
  Breadcrumbs,
  ViamSwitch,
  ViamIcon,
  RadioButtons,
  Tabs,
  Tab,
  ViamBadge,
  ViamButton,
  Container,
  Grid,
  Range,
  ViamInput,
  ViamBase,
  KeyboardInput,
  Camera,
  ViamSelect,
  ViamInfoButton,
  Popper,
  NumberInput,
  Slam,
};

Object.keys(Components).forEach((name) => {
  Vue.component(name, Components[name]);
});

Vue.directive("click-outside", ClickOutside);

export default Components;
