import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from "path"
import tailwindcss from "@tailwindcss/vite"

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react({
      jsxRuntime: 'automatic',
    }),
    tailwindcss()
  ],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        'src/**/*.test.{ts,tsx}',
        'src/**/__tests__/**',
        'src/test-setup.ts',
        'src/**/*.d.ts',
      ],
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
    dedupe: ['react', 'react-dom'],
  },
  server: {
    port: 3500,
    strictPort: true,
    headers: {
      // Always load the current HTML document during development. Vite's
      // versioned module assets can still be cached normally.
      'Cache-Control': 'no-store',
    },
    proxy: {
      '/v1': {
        target: 'http://localhost:3501',
        changeOrigin: true,
        secure: false,
        ws: true,
      },
      '/media': {
        target: 'http://localhost:3501',
        changeOrigin: true,
        secure: false,
      },
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false,
    minify: 'esbuild', // Use esbuild instead of terser
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['react', 'react-dom'],
          router: ['react-router'],
        },
      },
    },
  },
  base: '/',
})
