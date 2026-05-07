import { defineStore } from "pinia";

export const useSessionStore = defineStore("session", {
  state: () => ({
    userId: "",
    token: "",
  }),
  actions: {
    login(userId: string, token: string) {
      this.userId = userId;
      this.token = token;
    },
    logout() {
      this.userId = "";
      this.token = "";
    },
  },
});
