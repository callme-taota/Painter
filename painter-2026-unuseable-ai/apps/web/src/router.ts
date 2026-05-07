import { createRouter, createWebHistory } from "vue-router";
import EntryPage from "./views/EntryPage.vue";
import ArticleListPage from "./views/ArticleListPage.vue";
import ArticleDetailPage from "./views/ArticleDetailPage.vue";
import TagListPage from "./views/TagListPage.vue";
import CategoryListPage from "./views/CategoryListPage.vue";
import UserPage from "./views/UserPage.vue";
import FollowPage from "./views/FollowPage.vue";
import LoginPage from "./views/LoginPage.vue";
import RegisterPage from "./views/RegisterPage.vue";
import DashboardPage from "./views/DashboardPage.vue";
import EditArticlePage from "./views/EditArticlePage.vue";
import AdminRedirectPage from "./views/AdminRedirectPage.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: EntryPage },
    { path: "/tag", component: TagListPage },
    { path: "/category", component: CategoryListPage },
    { path: "/articlelist", component: ArticleListPage },
    { path: "/articles", component: ArticleListPage },
    { path: "/article", component: ArticleDetailPage, props: (route) => ({ id: route.query.ArticleID ?? "a-1" }) },
    { path: "/articles/:id", component: ArticleDetailPage },
    { path: "/tags", component: TagListPage },
    { path: "/categories", component: CategoryListPage },
    { path: "/user", component: UserPage, props: (route) => ({ id: route.query.UserID ?? "u-admin" }) },
    { path: "/users/:id", component: UserPage },
    { path: "/follow", component: FollowPage },
    { path: "/users/:id/follows", component: FollowPage },
    { path: "/login", component: LoginPage },
    { path: "/register", component: RegisterPage },
    { path: "/dashboard", component: DashboardPage },
    { path: "/me", component: DashboardPage },
    { path: "/admin", component: AdminRedirectPage },
    { path: "/editarticle", component: EditArticlePage },
    { path: "/editor/new", component: EditArticlePage },
    { path: "/editor/:id", component: EditArticlePage },
  ],
});
