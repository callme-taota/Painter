import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    
    {
      path: '/',
      name: 'entry',
      component: () => import("@/views/entry.vue")
    },
    {
      path: '/tag',
      name: 'tagList',
      component: () => import("@/views/list/tagList.vue")
    },
    {
      path: '/category',
      name: 'categoryList',
      component: () => import("@/views/list/categoryList.vue")
    },
    {
      path: '/articlelist',
      name: 'articleList',
      component: () => import("@/views/list/articleList.vue")
    },
    {
      path: '/login',
      name: 'login',
      component: () => import("@/views/auth/login.vue")
    },
    {
      path: '/register',
      name: 'register',
      component: () => import("@/views/auth/register.vue")
    },
    {
      path: '/user',
      name: 'user',
      component: () => import("@/views/user.vue"),
    },
    {
      path: '/follow',
      name: 'follow',
      component: () => import('@/views/list/followList.vue'),
    },
    {
      path: '/article',
      name: 'article',
      component: () => import("@/views/article/article.vue")
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import("@/views/admin/admin.vue")
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import("@/views/dashboard.vue")
    },
    {
      path: '/editarticle',
      name: 'editarticle',
      component: () => import("@/views/article/editarticle.vue")
    },
  ]
})

export default router
