import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/home.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    
    {
      path: '/',
      name: 'home',
      component: HomeView
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
      path: '/signup',
      name: 'signup',
      component: () => import("@/views/signup.vue")
    },
    {
      path: '/user',
      name: 'user',
      component: () => import("@/views/user.vue"),
    },
    {
      path: '/follow',
      name: 'follow',
      component: () => import("@/views/followList.vue")
    },
    {
      path: '/article',
      name: 'article',
      component: () => import("@/views/article.vue")
    },
    
    
    // {
    //   path: '/about',
    //   name: 'about',
    //   // route level code-splitting
    //   // this generates a separate chunk (About.[hash].js) for this route
    //   // which is lazy-loaded when the route is visited.
    //   component: () => import('../views/AboutView.vue')
    // }
  ]
})

export default router
