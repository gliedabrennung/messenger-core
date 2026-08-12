import { useState, useEffect, useCallback } from 'react';
import axios from 'axios';
import { api } from '@/api';
import { useChatStore } from '@/store/chatStore';
import type { Chat, ChatListResponse } from '@/types';

export function useChats() {
  const [chats, setChats] = useState<Chat[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const setRecentChats = useChatStore((s) => s.setRecentChats);
  const refreshToken = useChatStore((s) => s.chatRefreshToken);

  const load = useCallback(
    (signal?: AbortSignal) =>
      api
        .get<ChatListResponse>('/chats', { signal })
        .then((res) => {
          if (signal?.aborted) return;
          const list = res.data.chats || [];
          setChats(list);
          setRecentChats(list.map((c) => ({ id: c.peer_id, username: c.peer_username })));
        })
        .catch((err: unknown) => {
          if (!axios.isCancel(err)) console.error(err);
        })
        .finally(() => {
          if (!signal?.aborted) setIsLoading(false);
        }),
    [setRecentChats]
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load, refreshToken]);

  return { chats, isLoading, refresh: load };
}
