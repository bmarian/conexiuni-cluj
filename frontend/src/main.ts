// First import: it takes its token out of the URL before the router reads it.
import { arriveFromLegacyHost, leaveLegacyHost } from "./utils/hostMove";
import { createApp } from "vue";
import { createHead } from "@unhead/vue/client";
import { createPinia } from "pinia";
import { registerSW } from "virtual:pwa-register";

import App from "./App.vue";
import router from "./router";
import { useUserStore } from "./stores/user";
import { useSettingsStore } from "./stores/settings";
import { useFavoritesStore } from "./stores/favorites";
import { usePushStore } from "./stores/push";
import { useRemindersStore } from "./stores/reminders";
import { useRoutesApi } from "./composables/useRoutesApi";
import { useStopsApi } from "./composables/useStopsApi";
import { startNetwork } from "./utils/network";
import "./main.css";
import "./styles/arcade.css";
import "./styles/legacy-blue.css";
import "@fontsource-variable/archivo/wdth.css";
import "@fontsource/libre-caslon-text/400.css";
import "@fontsource/libre-caslon-text/400-italic.css";
import "@fontsource/libre-caslon-text/700.css";
import "./styles/paper.css";
import { i18n } from "./i18n";
import { adoptPreCitySaves, apiPath, citiesReady } from "./utils/city";
import { lockIOSPageZoom } from "./utils/ios";

lockIOSPageZoom();

const sendPWAInstallEvent = () => {
  const body = JSON.stringify({ metric: "pwa_install", key: "appinstalled" });
  const blob = new Blob([body], { type: "application/json" });
  if (navigator.sendBeacon?.(apiPath("stats/event"), blob)) return;
  void fetch(apiPath("stats/event"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body,
    keepalive: true,
  }).catch(() => {});
};

function start() {
  adoptPreCitySaves();
  registerSW({
    immediate: true,
    onRegisteredSW(_url, registration) {
      if (!registration) return;
      const checkForUpdate = () => {
        if (!document.hidden) registration.update().catch(() => {});
      };
      checkForUpdate();
      document.addEventListener("visibilitychange", checkForUpdate);
    },
  });

  window.addEventListener("appinstalled", sendPWAInstallEvent);

  const isAdminPath = window.location.pathname === "/admin";

  if (!isAdminPath) {
    void useRoutesApi().fetchRoutes();
    void useStopsApi().fetchStops();
    void startNetwork();
  }

  const app = createApp(App);
  const pinia = createPinia();

  app.use(i18n);
  app.use(pinia);
  app.use(router);
  app.use(createHead());

  const settingsStore = useSettingsStore(pinia);
  i18n.global.locale.value = settingsStore.locale;

  const userStore = useUserStore(pinia);
  userStore.startTimeTracker();

  const favoritesStore = useFavoritesStore(pinia);
  void favoritesStore.hydrate().then(() => {
    if (!isAdminPath) favoritesStore.preloadFavorites();
  });

  if (!isAdminPath) {
    void usePushStore(pinia).sync();
    useRemindersStore(pinia);
  }

  // Sent by push-sw.js when a notification is tapped while the app is already open.
  navigator.serviceWorker?.addEventListener("message", (event) => {
    const url = event.data?.type === "open-url" ? event.data.url : null;
    if (typeof url === "string" && url.startsWith("/")) void router.push(url);
  });

  void citiesReady.then(() => router.isReady()).then(() => app.mount("#app"));
}

// The stores read what the move writes, so they start after it.
void leaveLegacyHost()
  .catch(() => false)
  .then(async (leaving) => {
    if (leaving) return;
    await arriveFromLegacyHost();
    start();
  });
