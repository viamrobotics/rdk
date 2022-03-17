import { enableAutoDestroy, shallowMount } from "@vue/test-utils";
import Icon from "@/components/ViamIcon.vue";

describe("ViamIcon", () => {
  enableAutoDestroy(afterEach);

  it("has html structure", async () => {
    const wrapper = shallowMount(Icon, {
      propsData: {
        name: "user",
      },
    });

    expect(wrapper.element.tagName).toBe("svg");
    expect(wrapper.attributes("fill")).toBe("none");
    expect(wrapper.attributes("stroke")).toBe("currentColor");
  });

  it("do not content inside a slot", async () => {
    const wrapper = shallowMount(Icon, {
      propsData: {
        name: "user",
      },
      slots: {
        default: "<section>content</section>",
      },
    });

    expect(wrapper.find("section").exists()).toBe(false);
    expect(wrapper.text()).toBe("");
  });
});
