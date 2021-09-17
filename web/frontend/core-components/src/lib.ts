import Vue, { VueConstructor } from "vue";
import "./styles.css";
import MotorDetail from "./components/MotorDetail.vue";

const Components: { [key: string]: VueConstructor<Vue> } = {
  MotorDetail,
};

Object.keys(Components).forEach((name) => {
  Vue.component(name, Components[name]);
});

export default Components;
