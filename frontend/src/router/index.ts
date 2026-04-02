import { createRouter, createWebHistory } from 'vue-router'
import RouteView from '@/views/RouteView.vue'
import NotFoundView from '@/views/NotFoundView.vue'

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
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: NotFoundView,
    },
  ],
})

router.beforeEach((to) => {
  if (to.name === 'route') {
    const routeShortName = to.params.routeShortName as string
    const uppercaseRouteShortName = routeShortName.toUpperCase()
    if (routeShortName !== uppercaseRouteShortName) {
      return{ name: 'route', params: { routeShortName: uppercaseRouteShortName } }
    }
  }
})

export default router
