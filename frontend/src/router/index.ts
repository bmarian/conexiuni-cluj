import {createRouter, createWebHistory} from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import StopView from '@/views/StopView.vue'
import RouteView from '@/views/RouteView.vue'
import RoutePlanningView from '@/views/RoutePlanningView.vue'
import NotFoundView from '@/views/NotFoundView.vue'

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
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: NotFoundView,
    },
  ],
})

export default router
