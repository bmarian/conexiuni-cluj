<template>
  <main class="not-found">
    <img src="/404.png" alt="404 - Page not found" class="not-found__image" />
  </main>
</template>

<style scoped>
.not-found {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 1rem;
}

.not-found__image {
  max-width: min(90vw, 720px);
  height: auto;
  display: block;
}
</style>
