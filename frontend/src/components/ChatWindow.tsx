import { useState, useEffect, useRef, useCallback, type FC, type SyntheticEvent } from 'react';
import { Send, MessageSquare } from 'lucide-react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useAuthStore } from '@/store/authStore';
import { useChatStore } from '@/store/chatStore';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useChatMessages } from '@/hooks/useChatMessages';
import { MessageBubble } from '@/components/MessageBubble';
import { Avatar } from '@/components/ui/Avatar';
import { Input } from '@/components/ui/Input';
import { StatusDot } from '@/components/ui/StatusDot';

const LOAD_MORE_THRESHOLD_PX = 80;

export const ChatWindow: FC = () => {
  const user = useAuthStore((state) => state.user);
  const activePartner = useChatStore((state) => state.activePartner);
  const addMessage = useChatStore((state) => state.addMessage);

  const { messages, isLoading, hasMore, loadMore } = useChatMessages(activePartner?.id);
  const { sendMessage, status } = useWebSocket();
  const [text, setText] = useState('');

  const containerRef = useRef<HTMLDivElement>(null);
  const scrollHeightRef = useRef<number>(0);
  const prevPartnerIdRef = useRef<number | undefined>(undefined);
  const prevMessagesLenRef = useRef<number>(0);

  const rowVirtualizer = useVirtualizer({
    count: messages.length,
    getScrollElement: () => containerRef.current,
    estimateSize: () => 72,
    overscan: 10,
  });

  useEffect(() => {
    if (containerRef.current && scrollHeightRef.current > 0) {
      const delta = containerRef.current.scrollHeight - scrollHeightRef.current;
      containerRef.current.scrollTop = delta;
      scrollHeightRef.current = 0;
    }
  }, [messages]);

  useEffect(() => {
    const partnerChanged = prevPartnerIdRef.current !== activePartner?.id;
    const isNewMessage = messages.length > prevMessagesLenRef.current && !partnerChanged;

    if ((partnerChanged || isNewMessage) && messages.length > 0) {
      const frame = requestAnimationFrame(() => {
        rowVirtualizer.scrollToIndex(messages.length - 1, { align: 'end' });
      });
      prevPartnerIdRef.current = activePartner?.id;
      prevMessagesLenRef.current = messages.length;
      return () => cancelAnimationFrame(frame);
    }

    prevPartnerIdRef.current = activePartner?.id;
    prevMessagesLenRef.current = messages.length;
  }, [messages.length, activePartner?.id, rowVirtualizer]);

  const handleScroll = useCallback(() => {
    const el = containerRef.current;

    if (el && el.scrollTop <= LOAD_MORE_THRESHOLD_PX && hasMore && !isLoading) {
      scrollHeightRef.current = el.scrollHeight;
      loadMore();
    }
  }, [hasMore, isLoading, loadMore]);

  const handleSend = useCallback(
    (e: SyntheticEvent) => {
      e.preventDefault();
      const body = text.trim();
      if (!body || !activePartner || !user) return;

      const clientId = `c-${Date.now()}-${Math.random().toString(36).slice(2)}`;
      const sent = sendMessage(activePartner.id, body, clientId);

      addMessage(activePartner.id, {
        client_id: clientId,
        from_id: user.id,
        to_id: activePartner.id,
        content: body,
        created_at: new Date().toISOString(),
        isPending: sent,
        failed: !sent,
      });

      setText('');
    },
    [text, activePartner, user, sendMessage, addMessage]
  );

  if (!activePartner) {
    return (
      <div className="flex-1 bg-[var(--color-surface-primary)] flex flex-col items-center justify-center select-none animate-fade-in">
        <div className="w-20 h-20 rounded-full gradient-surface flex items-center justify-center mb-5 border border-[var(--color-border-primary)] shadow-lg">
          <MessageSquare size={32} className="text-[var(--color-accent-start)]" />
        </div>
        <h3 className="text-lg font-semibold text-[var(--color-text-primary)] mb-1">
          Welcome to Messenger
        </h3>
        <p className="text-sm text-[var(--color-text-muted)]">
          Select a conversation to start messaging
        </p>
      </div>
    );
  }

  const virtualItems = rowVirtualizer.getVirtualItems();

  return (
    <div className="flex-1 flex flex-col bg-[var(--color-surface-primary)] h-screen">
      <div className="p-4 border-b border-[var(--color-border-primary)] glass flex items-center gap-3">
        <Avatar username={activePartner.username} />
        <div>
          <div className="font-semibold text-[var(--color-text-primary)]">{activePartner.username}</div>
          <div className="text-xs text-[var(--color-text-muted)] flex items-center gap-1.5">
            <StatusDot status={status} />
            ID: {activePartner.id}
          </div>
        </div>
      </div>

      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto p-4"
      >
        {isLoading && (
          <div className="text-center text-xs text-[var(--color-text-muted)] py-2 animate-pulse-soft">
            Loading messages...
          </div>
        )}

        <div
          style={{
            height: `${rowVirtualizer.getTotalSize()}px`,
            width: '100%',
            position: 'relative',
          }}
        >
          {virtualItems.map((virtualItem) => {
            const msg = messages[virtualItem.index];
            return (
              <div
                key={msg.message_id || msg.client_id || `${msg.from_id}-${msg.created_at}`}
                data-index={virtualItem.index}
                ref={rowVirtualizer.measureElement}
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  transform: `translateY(${virtualItem.start}px)`,
                  paddingBottom: '12px',
                }}
              >
                <MessageBubble
                  message={msg}
                  isMe={msg.from_id === user?.id}
                />
              </div>
            );
          })}
        </div>
      </div>

      <div className="p-4 glass border-t border-[var(--color-border-primary)]">
        <form id="message-form" onSubmit={handleSend} className="flex gap-2">
          <Input
            id="message-input"
            type="text"
            placeholder="Write a message..."
            className="flex-1 !bg-[var(--color-surface-primary)] !border-[var(--color-border-primary)] !rounded-[var(--radius-full)] !px-5 !py-2.5 !text-sm"
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
          <button
            id="send-button"
            type="submit"
            disabled={!text.trim()}
            className="p-3 gradient-accent text-white rounded-full hover:opacity-90 transition-all duration-200 disabled:opacity-40 disabled:cursor-not-allowed flex-shrink-0 flex items-center justify-center shadow-sm hover:shadow-md active:scale-95"
          >
            <Send size={18} />
          </button>
        </form>
      </div>
    </div>
  );
};
