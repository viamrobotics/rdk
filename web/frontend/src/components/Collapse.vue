<template>
  <component
    :is="tag"
    :aria-disabled="disabled"
    :aria-expanded="collapsed ? 'false' : 'true'"
  >
    <div
      class="flex items-center border border-black p-2"
      :class="{
        'flex-row-reverse': iconLeft,
      }"
    >
      <div
        data-cy="collapse-container"
        class="flex-1 cursor-pointer"
        @click="toggleExpand()"
      >
        <slot />
      </div>

      <div
        :class="{
          'cursor-pointer': !disabled,
        }"
        @click="toggleExpand()"
      >
        <svg
          width="24"
          height="24"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-linejoin="round"
          stroke-linecap="round"
          fill="none"
          role="presentation"
          :class="[
            'stroke-2 w-5 h-5 transform transition-transform duration-200',
            iconLeft ? 'mr-2' : 'ml-2',
            { 'rotate-180': !collapsed },
          ]"
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </div>
    </div>

    <template v-if="$slots.summary">
      <slot name="summary" />
    </template>

    <div
      :class="[
        'transition-all duration-300 ease-in-out',
        { 'overflow-y-hidden': !expandedCompleted },
      ]"
      data-cy="collapse-content"
      :style="{ height: height }"
    >
      <div ref="content">
        <slot name="content" />
      </div>
    </div>
  </component>
</template>

<script>
const collapseDurationMs = 300;

// TODO: convert to the class type component

export default {
  props: {
    disabled: {
      default: false,
      type: Boolean,
    },
    expanded: {
      default: false,
      type: Boolean,
    },
    iconLeft: {
      default: false,
      type: Boolean,
    },
    tag: {
      default: "div",
      type: String,
    },
  },
  data() {
    return {
      collapsed: !this.expanded,
      expandedCompleted: false,
      height: 0,
    };
  },
  beforeMount() {
    window.addEventListener("resize", this.resizeContent);
  },
  beforeDestroy() {
    window.removeEventListener("resize", this.resizeContent);
  },
  mounted() {
    this.resizeContent();
  },
  methods: {
    toggleExpand() {
      if (this.disabled) return;
      this.expandedCompleted = false;
      if (this.collapsed) {
        setTimeout(() => {
          this.expandedCompleted = true;
        }, collapseDurationMs);
      }
      this.collapsed = !this.collapsed;
      this.$emit("toggle", this.collapsed);
      this.resizeContent();
    },
    resizeContent() {
      if (this.collapsed) {
        this.height = 0;
      } else {
        this.height = "auto";
      }
    },
  },
}
</script>
