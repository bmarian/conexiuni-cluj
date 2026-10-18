import {createRouter, createWebHistory} from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import {isShare, sharedDestination} from '@/utils/share.ts'
import {citiesLoaded, currentCity, knownCity, rememberCity, resolveCity} from '@/utils/city.ts'

const StopView = () => import('@/views/StopView.vue')
const RouteView = () => import('@/views/RouteView.vue')
const RoutePlanningView = () => import('@/views/RoutePlanningView.vue')
const NotFoundView = () => import('@/views/NotFoundView.vue')
const AdminView = () => import('@/views/AdminView.vue')
const SettingsView = () => import('@/views/SettingsView.vue')
const NavigationView = () => import('@/views/NavigationView.vue')
const AndroidView = () => import('@/views/AndroidView.vue')

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'landing',
      component: HomeView,
      beforeEnter: async () => `/${await resolveCity()}/`,
    },
    {
      path: '/android',
      name: 'android',
      component: AndroidView,
    },
    {
      path: '/admin',
      name: 'admin',
      component: AdminView,
    },
    {
      path: '/:city',
      children: [
        {
          path: '',
          name: 'home',
          component: HomeView,
        },
        {
          path: 'stop/:stopId',
          name: 'stop',
          component: StopView,
          props: true,
        },
        {
          path: 'route/:routeId/:direction',
          name: 'route',
          component: RouteView,
          props: true,
        },
        {
          path: 'plan',
          name: 'plan',
          component: RoutePlanningView,
          beforeEnter: (to) => (isShare(to.query) ? sharedDestination(to.query) : true),
        },
        {
          path: 'settings',
          name: 'settings',
          component: SettingsView,
        },
        {
          path: 'nav',
          name: 'nav',
          component: NavigationView,
        },
        {
          path: 'bsod',
          name: 'bsod',
          component: NotFoundView,
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      redirect: {name: 'home', params: {city: currentCity}},
    },
  ],
})

router.beforeEach(async (to) => {
  const slug = to.params.city
  if (typeof slug !== 'string') return true
  await citiesLoaded
  if (!knownCity(slug)) return `/${currentCity}/`
  if (slug !== currentCity) {
    // Every cache is keyed by the page's city, so another city needs a full load.
    rememberCity(slug)
    window.location.href = to.fullPath
    return false
  }
  rememberCity(slug)
  return true
})

export default router
