import { createPinia, setActivePinia } from "pinia";
import { describe, expect, it, beforeEach } from "vitest";
import { useSessionStore } from "../session";

describe("session store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it("login and logout", () => {
    const store = useSessionStore();
    store.login("u1", "t1");
    expect(store.userId).toBe("u1");
    store.logout();
    expect(store.userId).toBe("");
  });
});
