/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The Go backend. The API is called with relative URLs, so `npm run dev` and
// `npm run preview` forward `/api` to it.
const apiProxy = { '/api': 'http://localhost:8080' }

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: { proxy: apiProxy },
  preview: { proxy: apiProxy },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      // The terminal shows only files with missing coverage; summary.txt
      // lists every file.
      reporter: [
        'text',
        'html',
        ['text', { file: 'summary.txt', skipFull: false }],
      ],
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        'src/main.tsx',
        'src/test/**',
        'src/**/*.test.{ts,tsx}',
        'src/**/*.d.ts',
      ],
      thresholds: {
        statements: 85,
        branches: 85,
        functions: 85,
        lines: 85,
      },
    },
  },
})
