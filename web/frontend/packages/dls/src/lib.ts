import Vue, { VueConstructor } from "vue";
import "./assets/css/styles.css";
import "./index.css";
import MotorDetail from "./components/MotorDetail.vue";
import InputController from "./components/InputController.vue";
import WebGamepad from "./components/WebGamepad.vue";
import Camera from "./components/Camera.vue";
import Accordion from "./components/Accordion.vue";
import ToggleButton from "./components/ToggleButton.vue";
import VueAccordion from "./components/VueAccordion.vue";
import ViamButton from "./components/Button.vue";
import ViamInput from "./components/ViamInput.vue";
import Collapse from "./components/Collapse.vue";

const Components: { [key: string]: VueConstructor<Vue> } = {
    MotorDetail,
    ViamButton,
    InputController,
    WebGamepad,
    Camera,
    Accordion,
    ToggleButton,
    VueAccordion,
    ViamInput,
    Collapse
  };
  
  Object.keys(Components).forEach((name) => {
    Vue.component(name, Components[name]);
  });
  
  export default Components;