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
  // Every authenticated page is lazy-loaded, so most runtime deps are only
  // reachable through route chunks. Left to discover them on first visit, Vite
  // re-optimizes and forces a full reload mid-session, which can render a
  // component while React is momentarily null ("Cannot read properties of null
  // (reading 'useContext')") or 404 an in-flight chunk ("Failed to fetch
  // dynamically imported module"). Pre-bundling the whole runtime dep set makes
  // Vite optimize once at startup and never again mid-session.
  optimizeDeps: {
    include: [
      'react',
      'react-dom',
      'react-dom/client',
      'react-router',
      'react-i18next',
      'i18next',
      'i18next-browser-languagedetector',
      'lucide-react',
      'recharts',
      'cmdk',
      'sonner',
      'date-fns',
      'react-day-picker',
      'next-themes',
      'class-variance-authority',
      'clsx',
      'tailwind-merge',
      'country-flag-icons',
      'radix-ui',
      '@radix-ui/react-accordion',
      '@radix-ui/react-checkbox',
      '@radix-ui/react-context-menu',
      '@radix-ui/react-dialog',
      '@radix-ui/react-hover-card',
      '@radix-ui/react-label',
      '@radix-ui/react-popover',
      '@radix-ui/react-scroll-area',
      '@radix-ui/react-select',
      '@radix-ui/react-separator',
      '@radix-ui/react-slider',
      '@radix-ui/react-slot',
      '@radix-ui/react-switch',
      '@radix-ui/react-tabs',
      '@radix-ui/react-toggle',
      '@radix-ui/react-tooltip',
    ],
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
