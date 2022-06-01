import { enableAutoDestroy, mount } from "@vue/test-utils";
import ViamSelect from "@/components/ViamSelect.vue";
import Vue from "vue";

describe("ViamSelect", () => {
  enableAutoDestroy(afterEach);

  it("has html structure", async () => {
    const wrapper = mount({
      data() {
        return {
          value: 1,
          options: [
            { label: "No Camera", value: 1 },
            { label: "Camera1", value: 2 },
          ],
        };
      },
      template:
        '<div> <ViamSelect v-model="value" :options="options"></ViamSelect></div>',
      components: { ViamSelect },
    });
    const label = wrapper.find("[data-cy=selected-value]")
      .element as HTMLElement;
    expect(label.innerHTML).toBe("No Camera");
  });
  it("value can be changed via v-model", async () => {
    const wrapper = mount({
      data() {
        return {
          value: 1,
          options: [
            { label: "No Camera", value: 1 },
            { label: "Camera1", value: 2 },
          ],
        };
      },
      template:
        '<div> <ViamSelect v-model="value" :options="options"></ViamSelect></div>',
      components: { ViamSelect },
    });
    const label = wrapper.find("[data-cy=selected-value]")
      .element as HTMLElement;
    wrapper.setData({ value: 2 });
    await Vue.nextTick();
    expect(label.innerHTML).toBe("Camera1");
  });
  it("options can be shown via click and be selected", async () => {
    const wrapper = mount({
      data() {
        return {
          value: 2,
          options: [
            { label: "No Camera", value: 1 },
            { label: "Camera1", value: 2 },
          ],
        };
      },
      template:
        '<div> <ViamSelect v-model="value" :options="options"></ViamSelect></div>',
      components: { ViamSelect },
    });
    await wrapper.find("[data-cy=collapse-container]").trigger("click");
    const firstOption = wrapper.find("[data-cy=option]");
    expect(firstOption.element.innerHTML.trim()).toBe("No Camera");
    await firstOption.trigger("click");
    expect(wrapper.vm.$data.value).toBe(1);
  });
});
