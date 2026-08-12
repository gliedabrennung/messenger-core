export interface User {
  id: number;
  username: string;
}

export interface Message {
  chat_id?: string;
  message_id?: string;
  from_id: number;
  to_id: number;
  content: string;
  created_at?: string;

  client_id?: string;
  isPending?: boolean;
  failed?: boolean;
}

export interface ChatHistoryResponse {
  messages: Message[];
  next_cursor: string | null;
}

export type ConnectionStatus = 'connected' | 'connecting' | 'disconnected';

export interface Chat {
  chat_id: string;
  peer_id: number;
  peer_username: string;
  last_message: string;
  last_from_id: number;
  last_activity: string;
}

export interface ChatListResponse {
  chats: Chat[];
}
