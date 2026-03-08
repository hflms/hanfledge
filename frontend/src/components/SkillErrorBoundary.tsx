import { Component, ReactNode, ErrorInfo } from 'react';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
}

interface State {
  hasError: boolean;
  error?: Error;
}

/**
 * Error boundary for skill renderers.
 * Catches rendering errors and provides fallback UI.
 */
export class SkillErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('[SkillErrorBoundary] Caught error:', error, errorInfo);
    this.props.onError?.(error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback || (
        <div style={{ padding: '24px', textAlign: 'center' }}>
          <h3>⚠️ 技能加载失败</h3>
          <p>请刷新页面重试，或联系技术支持。</p>
          <details style={{ marginTop: '16px', textAlign: 'left' }}>
            <summary>错误详情</summary>
            <pre style={{ fontSize: '12px', overflow: 'auto' }}>
              {this.state.error?.message}
            </pre>
          </details>
        </div>
      );
    }

    return this.props.children;
  }
}
