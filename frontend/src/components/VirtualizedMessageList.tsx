import { useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import type { ChatMessage } from '@/lib/plugin/hooks';
import styles from './VirtualizedMessageList.module.css';

interface VirtualizedMessageListProps {
  messages: ChatMessage[];
  renderMessage: (message: ChatMessage) => React.ReactNode;
  estimateSize?: number;
}

/**
 * Virtualized message list for performance with large message counts.
 * Only renders visible messages in the viewport.
 */
export default function VirtualizedMessageList({
  messages,
  renderMessage,
  estimateSize = 100,
}: VirtualizedMessageListProps) {
  const parentRef = useRef<HTMLDivElement>(null);

  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: messages.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => estimateSize,
    overscan: 5,
  });

  return (
    <div ref={parentRef} className={styles.container}>
      <div
        style={{
          height: `${virtualizer.getTotalSize()}px`,
          width: '100%',
          position: 'relative',
        }}
      >
        {virtualizer.getVirtualItems().map(virtualRow => (
          <div
            key={virtualRow.key}
            data-index={virtualRow.index}
            ref={virtualizer.measureElement}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              transform: `translateY(${virtualRow.start}px)`,
            }}
          >
            {renderMessage(messages[virtualRow.index])}
          </div>
        ))}
      </div>
    </div>
  );
}
