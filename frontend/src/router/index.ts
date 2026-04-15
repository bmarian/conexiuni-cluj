import { createRouter, createWebHistory } from 'vue-router'
import StopView from "@/views/StopView.vue";
import RouteView from "@/views/RouteView.vue";

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
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
  ],
})

export default router
