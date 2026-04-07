import { createRouter, createWebHistory } from 'vue-router'
import StopView from "@/views/StopView.vue";

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/stop/:stopId',
      name: 'stop',
      component: StopView,
      props: true,
    }
  ],
})

export default router
