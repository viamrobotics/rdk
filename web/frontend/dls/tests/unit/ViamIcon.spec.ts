import { enableAutoDestroy, shallowMount } from "@vue/test-utils";
import Icon from "@/components/ViamIcon.vue";
import { mdiRotateLeft } from "@mdi/js";

describe("ViamIcon", () => {
  enableAutoDestroy(afterEach);

  it("has html structure", async () => {
    const wrapper = shallowMount(Icon, {
      propsData: {
        path: mdiRotateLeft,
      },
    });

    expect(wrapper.element.tagName).toBe("svg");
    expect(wrapper.attributes("viewBox")).toBe("0 0 24 24");
  });

  it("do not content inside a slot", async () => {
    const wrapper = shallowMount(Icon, {
      propsData: {
        path: mdiRotateLeft,
      },
      slots: {
        default: "<section>content</section>",
      },
    });

    expect(wrapper.find("section").exists()).toBe(false);
    expect(wrapper.text()).toBe("");
  });

  it("show title", async () => {
    const wrapper = shallowMount(Icon, {
      propsData: {
        path: mdiRotateLeft,
        title: "content",
      },
    });

    expect(wrapper.find("title").exists()).toBe(true);
    expect(wrapper.text()).toBe("content");
  });

  it("show description", async () => {
    const wrapper = shallowMount(Icon, {
      propsData: {
        path: mdiRotateLeft,
        description: "content",
      },
    });

    expect(wrapper.find("description").exists()).toBe(true);
    expect(wrapper.text()).toBe("content");
  });
});
