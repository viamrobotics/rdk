import Vue, { VueConstructor } from "vue";
import "./assets/css/styles.css";
import "./assets/css/fontawesome.min.css";
import MotorDetail from "./components/MotorDetail.vue";

const Components: { [key: string]: VueConstructor<Vue> } = {
  MotorDetail,
};

Object.keys(Components).forEach((name) => {
  Vue.component(name, Components[name]);
});

export default Components;
