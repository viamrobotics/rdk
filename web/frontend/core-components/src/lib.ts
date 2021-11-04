import Vue, { VueConstructor } from "vue";
import "./assets/css/styles.css";
import "./assets/css/fontawesome.min.css";
import "./assets/css/solid.min.css";
import "./assets/css/regular.min.css";
import MotorDetail from "./components/MotorDetail.vue";
import Gamepad from "./components/Gamepad.vue";

const Components: { [key: string]: VueConstructor<Vue> } = {
  MotorDetail,
  Gamepad,
};

Object.keys(Components).forEach((name) => {
  Vue.component(name, Components[name]);
});

export default Components;
