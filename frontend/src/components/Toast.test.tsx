import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ToastProvider, useToast } from './Toast';

// -- Test Trigger Component ----------------------------------------

function ToastTrigger({ message, variant }: { message: string; variant?: 'success' | 'error' | 'warning' | 'info' }) {
  const { toast } = useToast();
  return <button onClick={() => toast(message, variant)}>Show Toast</button>;
}

// -- Helpers -------------------------------------------------------

function renderWithProvider(ui: React.ReactNode) {
  return render(<ToastProvider>{ui}</ToastProvider>);
}

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

// -- Tests ---------------------------------------------------------

describe('Toast', () => {
  it('renders toast message when triggered', async () => {
    const user = userEvent.setup();
    renderWithProvider(<ToastTrigger message="操作成功" variant="success" />);

    await user.click(screen.getByText('Show Toast'));

    expect(screen.getByText('操作成功')).toBeInTheDocument();
  });

  it('renders multiple toasts', () => {
    renderWithProvider(
      <>
        <ToastTrigger message="第一条" variant="info" />
        <ToastTrigger message="第二条" variant="error" />
      </>,
    );

    const buttons = screen.getAllByText('Show Toast');
    fireEvent.click(buttons[0]);
    fireEvent.click(buttons[1]);

    expect(screen.getByText('第一条')).toBeInTheDocument();
    expect(screen.getByText('第二条')).toBeInTheDocument();
  });

  it('toast disappears after timeout', () => {
    vi.useFakeTimers();
    renderWithProvider(<ToastTrigger message="消失测试" />);

    fireEvent.click(screen.getByText('Show Toast'));
    expect(screen.getByText('消失测试')).toBeInTheDocument();

    // Advance past TOAST_DURATION (4000ms)
    act(() => {
      vi.advanceTimersByTime(4500);
    });

    expect(screen.queryByText('消失测试')).not.toBeInTheDocument();
  });

  it('toast can be dismissed by clicking close button', () => {
    renderWithProvider(<ToastTrigger message="关闭测试" />);

    fireEvent.click(screen.getByText('Show Toast'));
    expect(screen.getByText('关闭测试')).toBeInTheDocument();

    // Click the close button (×)
    const closeButton = screen.getByText('×');
    fireEvent.click(closeButton);

    expect(screen.queryByText('关闭测试')).not.toBeInTheDocument();
  });

  it('useToast throws when used outside provider', () => {
    // Suppress console.error from React error boundary
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});

    function BadComponent() {
      useToast();
      return null;
    }

    expect(() => render(<BadComponent />)).toThrow('useToast must be used within <ToastProvider>');
    spy.mockRestore();
  });

  it('defaults to info variant when no variant specified', () => {
    const { container } = renderWithProvider(<ToastTrigger message="默认" />);

    fireEvent.click(screen.getByText('Show Toast'));

    // The info icon (ℹ) should be rendered
    const toast = container.querySelector('[class*="info"]');
    expect(toast).toBeInTheDocument();
  });
});
