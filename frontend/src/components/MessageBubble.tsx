import { memo, useMemo, type FC } from 'react';
import clsx from 'clsx';
import type { Message } from '@/types';

interface MessageBubbleProps {
  message: Message;
  isMe: boolean;
}

const MAX_DISPLAY_LENGTH = 4000;

function sanitizeContent(raw: string): string {
  if (raw.length > MAX_DISPLAY_LENGTH) {
    return raw.slice(0, MAX_DISPLAY_LENGTH) + '…';
  }
  return raw;
}

const MessageBubbleBase: FC<MessageBubbleProps> = ({ message, isMe }) => {
  const content = sanitizeContent(message.content);

  const formattedTime = useMemo(() => {
    if (!message.created_at) return '';
    return new Date(message.created_at).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
    });
  }, [message.created_at]);

  return (
    <div className={clsx('flex animate-fade-in', isMe ? 'justify-end' : 'justify-start')}>
      <div
        className={clsx(
          'max-w-[70%] rounded-2xl px-4 py-2.5 shadow-sm transition-opacity duration-200',
          isMe
            ? 'gradient-accent text-white rounded-br-sm'
            : 'bg-[var(--color-surface-secondary)] text-[var(--color-text-primary)] rounded-bl-sm border border-[var(--color-border-subtle)]',
          message.isPending && 'opacity-60',
          message.failed && 'opacity-70 ring-1 ring-[var(--color-danger)]'
        )}
      >
        <p className="text-sm break-words whitespace-pre-wrap leading-relaxed">{content}</p>
        <div
          className={clsx(
            'text-[10px] mt-1 text-right select-none',
            message.failed
              ? 'text-[var(--color-danger)]'
              : isMe
                ? 'text-indigo-200/70'
                : 'text-[var(--color-text-muted)]'
          )}
        >
          {message.failed ? 'Not delivered' : message.isPending ? 'Sending...' : formattedTime}
        </div>
      </div>
    </div>
  );
};

export const MessageBubble = memo(MessageBubbleBase);
