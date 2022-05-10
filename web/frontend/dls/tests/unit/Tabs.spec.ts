import { enableAutoDestroy, shallowMount } from "@vue/test-utils";
import Tabs from "@/components/Tabs.vue";

describe("Tabs", () => {
  enableAutoDestroy(afterEach);

  it("has html structure", async () => {
    const wrapper = shallowMount(Tabs);

    expect(wrapper.element.tagName).toBe("NAV");
    expect(wrapper.classes("bg-gray-100")).toBe(true);
  });

  it("renders content inside the slot", async () => {
    const wrapper = shallowMount(Tabs, {
      slots: {
        default: "<span>content</span>",
      },
    });

    expect(wrapper.find("span").exists()).toBe(true);
    expect(wrapper.text()).toBe("content");
  });

  it("renders root element", async () => {
    const wrapper = shallowMount(Tabs, {
      propsData: {
        tag: "section",
      },
    });

    expect(wrapper.element.tagName).toBe("SECTION");
  });

  it("should emit events", async () => {
    let called = 0;
    let event = null;
    const wrapper = shallowMount(Tabs, {
      listeners: {
        blur: (e: Event) => {
          event = e;
          called += 1;
        },
        click: (e: Event) => {
          event = e;
          called += 1;
        },
        focus: (e: Event) => {
          event = e;
          called += 1;
        },
      },
    });

    expect(called).toBe(0);
    expect(event).toEqual(null);

    await wrapper.trigger("click");
    expect(called).toBe(1);
    expect(event).toBeInstanceOf(MouseEvent);

    wrapper.element.dispatchEvent(new Event("focus"));
    expect(called).toBe(2);
    expect(event).toBeInstanceOf(Event);

    wrapper.element.dispatchEvent(new Event("blur"));
    expect(called).toBe(3);
    expect(event).toBeInstanceOf(Event);
  });
});
