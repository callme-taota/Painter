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
      path: '/home',
      name: 'home',
      component: () => import("@/views/home.vue")
    },
    {
      path: '/tag',
      name: 'tagList',
      component: () => import("@/views/tagList.vue")
    },
    {
      path: '/category',
      name: 'categoryList',
      component: () => import("@/views/categoryList.vue")
    },
    {
      path: '/articlelist',
      name: 'articleList',
      component: () => import("@/views/articleList.vue")
    },
    {
      path: '/login',
      name: 'login',
      component: () => import("@/views/login.vue")
    },
    {
      path: '/register',
      name: 'register',
      component: () => import("@/views/register.vue")
    },
    {
      path: '/user',
      name: 'user',
      component: () => import("@/views/user.vue"),
    },
    {
      path: '/follow',
      name: 'follow',
      component: () => import('@/views/followList.vue'),
    },
    {
      path: '/article',
      name: 'article',
      component: () => import("@/views/article.vue")
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import("@/views/admin.vue")
    },

    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import("@/views/dashboard.vue")
    },
    {
      path: '/editarticle',
      name: 'editarticle',
      component: () => import("@/views/editarticle.vue")
    },
  ]
})

export default router
