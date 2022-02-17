<template>
  <div class="component">
    <div class="tab__header">
      <a href="#"
         class="tab__link p-4 block bg-blue-dark hover:bg-blue-darker no-underline text-white border-b-2 border-white flex justify-between"
         @click.prevent="active = !active">
        <strong>{{ title }}</strong>
        <span class="pill">Camera</span>
        <span class="down-Arrow" v-show="!active">&#9660;</span>
        <span class="up-Arrow" v-show="active">&#9650;</span>
      </a>
    </div>
    <div class="tab__content p-2" v-show="active">
      <slot/>
    </div>
  </div>
</template>

<script lang="ts">
import {Component, Prop, Vue, Watch} from "vue-property-decorator";
import "vue-class-component/hooks";

@Component
export default class Accordion extends Vue {
  @Prop() title!: string;
  active: string | undefined = true;
}
</script>

<style scoped>
</style>
