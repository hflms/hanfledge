import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import SystemSettingsPage from './page';

const mockApiFetch = vi.fn();
const mockToast = vi.fn();

vi.mock('@/lib/api', () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

vi.mock('@/components/Toast', () => ({
  useToast: () => ({ toast: mockToast }),
}));

vi.mock('@/lib/a11y', () => ({
  useModalA11y: () => ({ current: null }),
}));

describe('SystemSettingsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    mockApiFetch.mockImplementation(async (path: string, options?: RequestInit) => {
      if (path === '/system/config' && (!options || options.method === undefined)) {
        return {
          CHAT_MODELS: JSON.stringify([
            {
              id: 'chat-1',
              provider: 'ollama',
              model: 'qwen2.5:7b',
              apiKey: '',
              baseUrl: 'http://localhost:11434',
              isDefault: true,
            },
          ]),
          LLM_MODELS: 'qwen2.5:7b',
          LLM_PROVIDER: 'ollama',
          OLLAMA_MODEL: 'qwen2.5:7b',
          OLLAMA_BASE_URL: 'http://localhost:11434',
          EMBEDDING_MODELS: JSON.stringify([]),
        };
      }

      if (path === '/system/config' && options?.method === 'PUT') {
        return { message: '配置更新成功' };
      }

      if (path === '/system/config/test-embedding-model') {
        return { message: 'ok' };
      }

      throw new Error(`Unhandled apiFetch call: ${path}`);
    });
  });

  it('persists chat model config immediately when modal confirm is clicked', async () => {
    render(<SystemSettingsPage />);

    await screen.findByText('系统 AI 设置');

    await userEvent.click(screen.getByRole('button', { name: '添加模型' }));

    const modelInput = screen.getByPlaceholderText('例如：qwen2.5:7b');
    const baseUrlInput = screen.getByPlaceholderText('http://localhost:11434');

    await userEvent.clear(modelInput);
    await userEvent.type(modelInput, 'qwen2.5:14b');

    await userEvent.clear(baseUrlInput);
    await userEvent.type(baseUrlInput, 'http://localhost:11435');

    await userEvent.click(screen.getByRole('button', { name: '确认' }));

    await waitFor(() => {
      const putCall = mockApiFetch.mock.calls.find(
        ([path, options]) => path === '/system/config' && options?.method === 'PUT',
      );

      expect(putCall).toBeTruthy();

      const payload = JSON.parse(putCall?.[1]?.body as string) as {
        CHAT_MODELS: string;
        LLM_MODELS: string;
      };
      const chatModels = JSON.parse(payload.CHAT_MODELS) as Array<{ model: string; baseUrl: string }>;

      expect(payload.LLM_MODELS).toContain('qwen2.5:14b');
      expect(chatModels).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            model: 'qwen2.5:14b',
            baseUrl: 'http://localhost:11435',
          }),
        ]),
      );
    });

    expect(mockToast).toHaveBeenCalledWith('模型配置已保存', 'success');
  });
});
