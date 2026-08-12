import { useState, useEffect, useCallback, useRef } from 'react';
import { api } from '@/api';
import { useChatStore, MAX_MESSAGES_PER_CHAT } from '@/store/chatStore';
import type { ChatHistoryResponse, Message } from '@/types';

const EMPTY_MESSAGES: Message[] = [];
const PAGE_SIZE = 50;

export function useChatMessages(partnerId: number | undefined) {
  const setMessages = useChatStore((s) => s.setMessages);
  const prependMessages = useChatStore((s) => s.prependMessages);
  const messages = useChatStore((s) => (partnerId ? s.messages[partnerId] || EMPTY_MESSAGES : EMPTY_MESSAGES));
  const [isLoading, setIsLoading] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const cursorRef = useRef<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const fetchMessages = useCallback(
    async (pid: number, cursor?: string) => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;

      setIsLoading(true);
      try {
        const url = `/messages?partner_id=${pid}&limit=${PAGE_SIZE}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`;
        const res = await api.get<ChatHistoryResponse>(url, { signal: controller.signal });
        if (controller.signal.aborted) return;

        const fetched = (res.data.messages || []).reverse();
        if (cursor) {
          prependMessages(pid, fetched);
        } else {
          setMessages(pid, fetched);
        }

        cursorRef.current = res.data.next_cursor;

        const loaded = (useChatStore.getState().messages[pid] || []).length;
        setHasMore(!!res.data.next_cursor && loaded < MAX_MESSAGES_PER_CHAT);
      } catch (err: any) {
        if (err.name !== 'CanceledError') console.error(err);
      } finally {
        if (!controller.signal.aborted) setIsLoading(false);
      }
    },
    [setMessages, prependMessages]
  );

  useEffect(() => {
    if (!partnerId) return;
    cursorRef.current = null;
    setHasMore(true);
    fetchMessages(partnerId);
    return () => abortRef.current?.abort();
  }, [partnerId, fetchMessages]);

  const loadMore = useCallback(() => {
    if (hasMore && !isLoading && cursorRef.current && partnerId) {
      fetchMessages(partnerId, cursorRef.current);
    }
  }, [hasMore, isLoading, partnerId, fetchMessages]);

  return { messages, isLoading, hasMore, loadMore };
}
