import { createRouter, createWebHistory } from "vue-router";
import AdminHomePage from "./views/AdminHomePage.vue";
import SiteSettingsPage from "./views/SiteSettingsPage.vue";
import TagManagePage from "./views/TagManagePage.vue";
import CategoryManagePage from "./views/CategoryManagePage.vue";
import UserPermissionPage from "./views/UserPermissionPage.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: AdminHomePage },
    { path: "/settings/site", component: SiteSettingsPage },
    { path: "/content/tags", component: TagManagePage },
    { path: "/content/categories", component: CategoryManagePage },
    { path: "/users/permissions", component: UserPermissionPage },
  ],
});
