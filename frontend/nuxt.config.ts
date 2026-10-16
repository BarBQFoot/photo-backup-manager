// https://nuxt.com/docs/api/configuration/nuxt-config
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath } from 'node:url';
export default defineNuxtConfig({
  ssr: false,
  router: {
    options: { hashMode: true },
  },
  // Only production builds write to the folder embedded by Go.
  // Nuxt dev cleans its output directory when starting or restarting.
  $production: {
    nitro: {
      output: { publicDir: fileURLToPath(new URL('./dist', import.meta.url)) },
    },
  },
  app: { head: { title: 'Photo Backup Manager', htmlAttrs: { lang: 'th' } } },
  vite: {
    plugins: [tailwindcss() as any],
  },
  css: ['~/assets/css/main.css'],
});
