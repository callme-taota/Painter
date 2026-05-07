import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import EntryPage from "../EntryPage.vue";

describe("EntryPage", () => {
  it("renders title", () => {
    const wrapper = mount(EntryPage);
    expect(wrapper.text()).toContain("Painter 2026");
  });
});
