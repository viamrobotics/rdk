<template>
    <collapse>
        <template v-slot:content>
            <div class="flex flex-col">
                <div :key="option[valueKey]" v-for="option in options" class="cursor-pointer">
                </div>
            </div>
        </template>
    </collapse>
</template>
<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import Collapse from "./Collapse.vue";

@Component({
  components: {
    Collapse
  },
})
export default class ViamRange extends Vue {
  @Prop({ default: "" }) name!: string;
  @Prop({ default: "DefaultId" }) id!: string;
  @Prop({ default: "id" }) valueKey!: string;

  @Prop({ required: true }) value!: number;
  @Prop({ required: true, type: Array }) options!: object[];

  get innerValue(): number {
    return this.value;
  }
  set innerValue(value: number) {
    this.$emit("input", value);
  }
}
</script>
<style>
</style>
