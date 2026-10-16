import { createRouter, createWebHistory } from 'vue-router'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: () => import('@/pages/HomePage.vue') },
    { path: '/explore', name: 'explore', component: () => import('@/pages/ExplorePage.vue') },
    { path: '/map', name: 'map', component: () => import('@/pages/MapPage.vue') },
    { path: '/map2', name: 'map2', component: () => import('@/pages/CitiesMapPage.vue') },
    { path: '/favorites', name: 'favorites', component: () => import('@/pages/FavoritesPage.vue') },
  ],
})
