import Vue, { VueConstructor } from "vue";
import "./assets/css/styles.css";
import "./assets/css/fontawesome.min.css";
import "./assets/css/solid.min.css";
import "./assets/css/regular.min.css";
import MotorDetail from "./components/MotorDetail.vue";
import InputController from "./components/InputController.vue";
import WebGamepad from "./components/WebGamepad.vue";

const Components: { [key: string]: VueConstructor<Vue> } = {
  MotorDetail,
  InputController,
  WebGamepad,
};

Object.keys(Components).forEach((name) => {
  Vue.component(name, Components[name]);
});

export default Components;
