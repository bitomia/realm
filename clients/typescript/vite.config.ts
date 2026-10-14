/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import { resolve } from 'node:path';
import { builtinModules } from 'node:module';
import dts from 'unplugin-dts/vite'

export default defineConfig({
  plugins: [dts({ include: ['src'], exclude: ['src/**/*.test.ts'], bundleTypes: true })],
  build: {
    target: 'node22',
    lib: {
      entry: resolve(import.meta.dirname, 'src/index.ts'),
      name: 'RealmClient',
      fileName: 'index',
      formats: ['es', 'cjs'],
    },
    rollupOptions: {
      external: [...builtinModules, /^node:/],
    },
  },
  test: {
    includeSource: ['src/**/*.{js,ts}'],
    include: ['tests/**'],
  },
});
