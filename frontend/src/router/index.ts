import {createRouter, createWebHistory} from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import {isShare, sharedDestination} from '@/utils/share.ts'
import {citiesReady, currentCity, knownCity, locatedCity, openingCity, setCity} from '@/utils/city.ts'

const StopView = () => import('@/views/StopView.vue')
const StopsView = () => import('@/views/StopsView.vue')
const RouteView = () => import('@/views/RouteView.vue')
const RoutePlanningView = () => import('@/views/RoutePlanningView.vue')
const AdminView = () => import('@/views/AdminView.vue')
const SettingsView = () => import('@/views/SettingsView.vue')
const AndroidView = () => import('@/views/AndroidView.vue')
const NewsView = () => import('@/views/NewsView.vue')
const DestinationsView = () => import('@/views/DestinationsView.vue')
const DestinationView = () => import('@/views/DestinationView.vue')

let uaVisualTransition = false
window.addEventListener('popstate', (e) => {
  uaVisualTransition = e.hasUAVisualTransition === true
})

export function takeUAVisualTransition(): boolean {
  const value = uaVisualTransition
  uaVisualTransition = false
  return value
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'landing',
      component: HomeView,
      beforeEnter: async () => {
        await citiesReady
        const slug = openingCity()
        void Promise.all([locatedCity(), router.isReady()]).then(([nearest]) => {
          if (nearest && nearest !== slug && currentCity.value === slug && router.currentRoute.value.name === 'home') {
            void router.replace(`/${nearest}/`)
          }
        })
        return `/${slug}/`
      },
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
      path: '/plan',
      redirect: (to) => ({path: `/${currentCity.value}/plan`, query: to.query, hash: to.hash}),
    },
    {
      // Sync QR codes name no city.
      path: '/settings',
      redirect: (to) => ({
        path: `/${currentCity.value}/settings`,
        query: to.query,
        hash: to.hash,
      }),
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
          path: 'stops/:stopIds',
          name: 'stops',
          component: StopsView,
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
          path: 'news',
          name: 'news',
          component: NewsView,
        },
        {
          path: 'destinations',
          name: 'destinations',
          component: DestinationsView,
        },
        {
          path: 'to/:slug',
          name: 'destination',
          component: DestinationView,
          props: true,
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      redirect: () => ({name: 'home', params: {city: currentCity.value}}),
    },
  ],
})

router.beforeEach(async (to) => {
  const slug = to.params.city
  if (typeof slug !== 'string') return true
  await citiesReady
  if (!knownCity(slug)) return '/'
  setCity(slug)
  return true
})

export default router
