<template>
  <svg
    class="stroke-2 inline-block"
    :class="{
      'h-3 w-3': size === 'sm',
      'h-4 w-4': size === 'base',
      'h-5 w-5': size === 'lg',
    }"
    fill="none"
    stroke="currentColor"
    viewBox="0 0 24 24"
    stroke-linecap="round"
    stroke-linejoin="round"
    xmlns="http://www.w3.org/2000/svg"
  >
    <g v-html="icons[name]" />
  </svg>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import featherIcons from "feather-icons/dist/icons.json";

@Component({
  components: {
    featherIcons,
  },
})
export default class ViamButton extends Vue {
  @Prop() name!: boolean;
  @Prop({ default: "base" })
  size!: string;

  icons: featherIcons;

  created(): void {
    this.icons = { ...featherIcons };
    if (!this.icons[this.name])
      throw new Error(`${this.name} icon is not available.`);
  }
}
</script>
