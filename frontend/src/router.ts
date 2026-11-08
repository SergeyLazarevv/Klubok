import { createRouter, createWebHistory } from 'vue-router'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: () => import('@/pages/HomePage.vue') },
    { path: '/explore', name: 'explore', component: () => import('@/pages/ExplorePage.vue') },
    // Страница «Карта» убрана — поиск по карте теперь в «Карте» (/explore); старые ссылки ведут туда
    { path: '/map', redirect: (to) => ({ path: '/explore', hash: to.hash }) },
    { path: '/map2', name: 'map2', component: () => import('@/pages/CitiesMapPage.vue') },
    { path: '/favorites', name: 'favorites', component: () => import('@/pages/FavoritesPage.vue') },
    { path: '/account', name: 'account', component: () => import('@/pages/AccountPage.vue') },
  ],
})
