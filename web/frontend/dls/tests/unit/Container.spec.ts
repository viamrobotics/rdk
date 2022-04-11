import { enableAutoDestroy, shallowMount } from "@vue/test-utils";
import Container from "@/components/Container.vue";

describe("Container", () => {
  enableAutoDestroy(afterEach);

  it("has html structure", async () => {
    const wrapper = shallowMount(Container);

    expect(wrapper.element.tagName).toBe("DIV");
    expect(wrapper.classes("container")).toBe(true);
  });

  it("render content inside a slot", async () => {
    const wrapper = shallowMount(Container, {
      slots: {
        default: "<span>content</span>",
      },
    });

    expect(wrapper.find("span").exists()).toBe(true);
    expect(wrapper.text()).toBe("content");
  });

  it("render root element", async () => {
    const wrapper = shallowMount(Container, {
      propsData: {
        tag: "section",
      },
    });

    expect(wrapper.element.tagName).toBe("SECTION");
  });
});
