import {createRouter, createWebHistory} from 'vue-router'
import HomeView from '@/views/HomeView.vue'

const StopView = () => import('@/views/StopView.vue')
const RouteView = () => import('@/views/RouteView.vue')
const RoutePlanningView = () => import('@/views/RoutePlanningView.vue')
const NotFoundView = () => import('@/views/NotFoundView.vue')
const AdminView = () => import('@/views/AdminView.vue')

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/stop/:stopId',
      name: 'stop',
      component: StopView,
      props: true,
    },
    {
      path: '/route/:routeId/:direction',
      name: 'route',
      component: RouteView,
      props: true,
    },
    {
      path: '/plan',
      name: 'plan',
      component: RoutePlanningView,
    },
    {
      path: '/admin',
      name: 'admin',
      component: AdminView,
    },
    {
      path: '/bsod',
      name: 'bsod',
      component: NotFoundView,
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      redirect: {name: 'home'},
    },
  ],
})

export default router
