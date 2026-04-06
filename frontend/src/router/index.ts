import { createRouter, createWebHistory } from 'vue-router'
import RouteView from '@/views/RouteView.vue'
import StationView from "@/views/StationView.vue";

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/route/:routeShortName',
      name: 'route',
      component: RouteView,
      props: true,
    },
    {
      path: '/station/:stationId',
      name: 'station',
      component: StationView,
      props: true,
    }
  ],
})

export default router
