import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  clearScreen: false,
  build: {
    // Keep frontend/dist/.gitkeep so the tracked placeholder survives builds.
    emptyOutDir: false,
  },
})
