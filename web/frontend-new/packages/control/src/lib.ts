import Vue, { VueConstructor } from "vue";
import "./assets/css/styles.css";
import "./index.css";

const Components: { [key: string]: VueConstructor<Vue> } = {
};
  
  Object.keys(Components).forEach((name) => {
    Vue.component(name, Components[name]);
  });
  
  export default Components;