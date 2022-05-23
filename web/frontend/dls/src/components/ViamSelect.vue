<template>
  <collapse ref="collapse">
    <span data-cy="selected-value">{{ selectedLabel }}</span>
    <template v-slot:content>
      <div
        data-cy="options-container"
        class="flex flex-col border-l border-r border-b border-black"
      >
        <div
          data-cy="option"
          @click="select(option[valueKey])"
          :key="option[valueKey]"
          v-for="option in options"
          class="cursor-pointer px-2 hover:bg-gray-100"
        >
          {{ option[labelKey] }}
        </div>
      </div>
    </template>
  </collapse>
</template>
<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import Collapse from "./Collapse.vue";
import { find } from "lodash";

@Component({
  components: {
    Collapse,
  },
})
export default class ViamRange extends Vue {
  @Prop({ default: "" }) name!: string;
  @Prop({ default: "DefaultId" }) id!: string;
  @Prop({ default: "value" }) valueKey!: string;
  @Prop({ default: "label" }) labelKey!: string;

  @Prop({ required: true }) value!: number | string;
  @Prop({ required: true, type: Array }) options!: Record<string, unknown>[];

  get innerValue(): number | string {
    return this.value;
  }
  set innerValue(value: number | string) {
    this.$emit("input", value);
  }

  get selectedLabel(): string | null {
    let foundOption = this.getOptionByKey(this.value) as Record<string, string>;
    if (!foundOption) return null;
    return foundOption[this.labelKey] || null;
  }

  getOptionByKey(key: number | string): Record<string, unknown> | undefined {
    return find(this.options, { [this.valueKey]: key });
  }
  select(key: string | number): void {
    this.innerValue = key;
    // eslint-disable-next-line
    const collapse: any = this.$refs.collapse;
    collapse.toggleExpand();
    this.$emit("selected", key);
  }
}
</script>
