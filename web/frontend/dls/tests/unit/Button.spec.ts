import { enableAutoDestroy, shallowMount } from "@vue/test-utils";
import ViamButton from "@/components/Button.vue";

describe("ViamButton", () => {
  enableAutoDestroy(afterEach);

  it("has html structure", async () => {
    const wrapper = shallowMount(ViamButton);

    expect(wrapper.element.tagName).toBe("BUTTON");
    expect(wrapper.attributes("type")).toBeDefined();
    expect(wrapper.attributes("type")).toBe("button");
    expect(wrapper.attributes("href")).not.toBeDefined();
    expect(wrapper.attributes("role")).not.toBeDefined();
    expect(wrapper.attributes("disabled")).not.toBeDefined();
    expect(wrapper.attributes("aria-busy")).not.toBeDefined();
    expect(wrapper.attributes("aria-disabled")).not.toBeDefined();
  });

  it("renders default slot content", async () => {
    const wrapper = shallowMount(ViamButton, {
      slots: {
        default: "<span>foobar</span>",
      },
    });

    expect(wrapper.find("span").exists()).toBe(true);
    expect(wrapper.text()).toBe("foobar");
  });

  it("renders custom root element", async () => {
    const wrapper = shallowMount(ViamButton, {
      propsData: {
        tag: "div",
      },
    });

    expect(wrapper.element.tagName).toBe("DIV");
    expect(wrapper.attributes("type")).not.toBeDefined();
  });

  it("has attribute disabled when disabled set", () => {
    const wrapper = shallowMount(ViamButton, {
      propsData: {
        disabled: true,
      },
    });

    expect(wrapper.attributes("disabled")).toBeDefined();
  });

  it("link has attribute aria-disabled when disabled set", async () => {
    const wrapper = shallowMount(ViamButton, {
      propsData: {
        tag: "a",
        disabled: true,
      },
    });

    expect(wrapper.element.tagName).toBe("A");
    expect(wrapper.attributes("aria-disabled")).toBeDefined();
    expect(wrapper.attributes("aria-disabled")).toBe("true");
  });

  it("has loading state", async () => {
    const wrapper = shallowMount(ViamButton, {
      propsData: {
        loading: true,
      },
    });

    expect(wrapper.attributes("disabled")).toBeDefined();
    expect(wrapper.attributes("aria-busy")).toBeDefined();
    expect(wrapper.attributes("aria-busy")).toBe("true");
  });

  it("should emit events", async () => {
    let called = 0;
    let event = null;
    const wrapper = shallowMount(ViamButton, {
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

    expect(wrapper.element.tagName).toBe("BUTTON");
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

  it("should not emit click event when clicked and disabled", async () => {
    let called = 0;
    const wrapper = shallowMount(ViamButton, {
      propsData: {
        disabled: true,
      },
      listeners: {
        click: () => {
          called += 1;
        },
      },
    });

    expect(called).toBe(0);
    await wrapper.trigger("click");
    expect(called).toBe(0);
  });
});
