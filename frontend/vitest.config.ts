import { defineConfig } from 'vitest/config';
import path from 'path';

export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      include: [
        'src/lib/plugin/hooks/useMessages.ts',
        'src/lib/plugin/hooks/useStateMachine.ts',
        'src/components/skill-ui/**/*.tsx',
        'src/lib/plugin/parsers.ts',
      ],
      exclude: ['**/*.test.{ts,tsx}', '**/*.d.ts', '**/index.ts'],
      thresholds: {
        lines: 60,
        functions: 60,
        branches: 60,
        statements: 60,
      },
    },
    css: {
      modules: {
        classNameStrategy: 'non-scoped',
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
});
