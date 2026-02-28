'use client';

/**
 * PluginSandbox — Two-tier isolation wrapper for plugins.
 *
 * - core/domain trust: Renders the plugin component directly
 *   (with scoped styling via a wrapper div). Shadow DOM is
 *   deferred to a future iteration since React 19 hydration
 *   inside Shadow DOM requires additional plumbing.
 *
 * - community trust: Renders inside an `<iframe sandbox>`
 *   with postMessage-based communication.
 *
 * Per design.md §7.14.
 */

import { useRef, useEffect, useCallback, type CSSProperties } from 'react';
import type { PluginRegistration, PluginMessage, AllowedHostMethod } from '@/lib/plugin/types';

// -- Props ----------------------------------------------------

interface PluginSandboxProps {
  /** The plugin registration to render. */
  plugin: PluginRegistration;
  /** Context data to pass to the plugin. */
  context?: Record<string, unknown>;
  /** Isolation strategy. */
  isolation: 'shadow-dom' | 'iframe';
}

// -- Host method handlers (community sandbox) -----------------

const HOST_METHODS: Record<AllowedHostMethod, (payload: unknown) => unknown> = {
  getStudentContext: () => ({}),
  getKnowledgePoint: () => ({}),
  sendMessageToAgent: () => ({ ok: true }),
  reportInteractionEvent: () => ({ ok: true }),
  requestUIToast: () => ({ ok: true }),
  getThemeVariables: () => {
    if (typeof document === 'undefined') return {};
    const style = getComputedStyle(document.documentElement);
    return {
      '--primary': style.getPropertyValue('--primary').trim(),
      '--bg-card': style.getPropertyValue('--bg-card').trim(),
      '--text-primary': style.getPropertyValue('--text-primary').trim(),
    };
  },
};

// -- Component ------------------------------------------------

export default function PluginSandbox({ plugin, context, isolation }: PluginSandboxProps) {
  // ---- Core / Domain: render directly -----------------------
  if (isolation === 'shadow-dom') {
    const { Component } = plugin;
    return (
      <div data-plugin-id={plugin.id} data-plugin-trust={plugin.trustLevel}>
        <Component {...(context || {})} />
      </div>
    );
  }

  // ---- Community: iframe sandbox ----------------------------
  return <IframeSandbox plugin={plugin} context={context} />;
}

// -- Iframe Sandbox Sub-Component -----------------------------

function IframeSandbox({
  plugin,
  context,
}: {
  plugin: PluginRegistration;
  context?: Record<string, unknown>;
}) {
  const iframeRef = useRef<HTMLIFrameElement>(null);

  // Handle incoming postMessage from the plugin iframe
  const handleMessage = useCallback(
    (event: MessageEvent) => {
      const iframe = iframeRef.current;
      if (!iframe || event.source !== iframe.contentWindow) return;

      const msg = event.data as PluginMessage;
      if (msg.type !== 'request' || !msg.method) return;

      const handler = HOST_METHODS[msg.method];
      if (!handler) {
        const response: PluginMessage = {
          type: 'response',
          id: msg.id,
          error: `Method not allowed: ${msg.method}`,
        };
        iframe.contentWindow?.postMessage(response, '*');
        return;
      }

      try {
        const result = handler(msg.payload);
        const response: PluginMessage = {
          type: 'response',
          id: msg.id,
          payload: result,
        };
        iframe.contentWindow?.postMessage(response, '*');
      } catch (err) {
        const response: PluginMessage = {
          type: 'response',
          id: msg.id,
          error: String(err),
        };
        iframe.contentWindow?.postMessage(response, '*');
      }
    },
    [],
  );

  useEffect(() => {
    window.addEventListener('message', handleMessage);
    return () => window.removeEventListener('message', handleMessage);
  }, [handleMessage]);

  // Send context to iframe once it loads
  const handleLoad = useCallback(() => {
    const iframe = iframeRef.current;
    if (!iframe?.contentWindow) return;
    const initMsg: PluginMessage = {
      type: 'event',
      id: 'init',
      method: undefined,
      payload: { pluginId: plugin.id, context },
    };
    iframe.contentWindow.postMessage(initMsg, '*');
  }, [plugin.id, context]);

  const iframeStyle: CSSProperties = {
    width: '100%',
    minHeight: 200,
    border: 'none',
    borderRadius: 8,
    background: 'transparent',
  };

  return (
    <div data-plugin-id={plugin.id} data-plugin-trust="community">
      <iframe
        ref={iframeRef}
        sandbox="allow-scripts"
        style={iframeStyle}
        title={`Plugin: ${plugin.name}`}
        onLoad={handleLoad}
      />
    </div>
  );
}
