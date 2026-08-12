import { useState, useCallback, type FC } from 'react';
import { Search, LogOut } from 'lucide-react';
import { useAuthStore } from '@/store/authStore';
import { useChatStore } from '@/store/chatStore';
import { useSearchUsers } from '@/hooks/useSearchUsers';
import { useChats } from '@/hooks/useChats';
import { PartnerItem } from '@/components/PartnerItem';
import { Avatar } from '@/components/ui/Avatar';
import { Input } from '@/components/ui/Input';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api';
import type { User } from '@/types';

export const Sidebar: FC = () => {
  const [search, setSearch] = useState('');
  const { results: searchResults, isLoading: isSearching } = useSearchUsers(search);
  const { chats, isLoading: isLoadingChats } = useChats();

  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);

  const activePartner = useChatStore((state) => state.activePartner);
  const setActivePartner = useChatStore((state) => state.setActivePartner);
  const recentChats = useChatStore((state) => state.recentChats);
  const addRecentChat = useChatStore((state) => state.addRecentChat);

  const navigate = useNavigate();

  const handleSelectPartner = useCallback(
    (partner: User) => {
      setActivePartner(partner);
      addRecentChat(partner);
      setSearch('');
    },
    [setActivePartner, addRecentChat]
  );

  const handleLogout = useCallback(async () => {
    try { await api.post('/auth/logout'); } catch { /* ignored */ }
    logout();
    navigate('/login');
  }, [logout, navigate]);

  const previews = new Map(chats.map((c) => [c.peer_id, c.last_message]));
  const displayList = search ? searchResults : recentChats;
  const isBusy = search ? isSearching : isLoadingChats;

  return (
    <div className="w-80 flex-shrink-0 bg-[var(--color-surface-secondary)] border-r border-[var(--color-border-primary)] flex flex-col h-screen">
      <div className="p-4 border-b border-[var(--color-border-primary)] glass">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Avatar username={user?.username} />
            <div>
              <div className="text-sm font-semibold text-[var(--color-text-primary)]">{user?.username}</div>
            </div>
          </div>
          <button
            id="logout-button"
            onClick={handleLogout}
            className="p-2 text-[var(--color-text-muted)] hover:text-[var(--color-danger)] hover:bg-[var(--color-danger-bg)] rounded-[var(--radius-sm)] transition-all duration-200"
            title="Sign out"
          >
            <LogOut size={18} />
          </button>
        </div>
      </div>

      <div className="p-3">
        <div className="relative">
          <Search
            className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--color-text-muted)]"
            size={16}
          />
          <Input
            id="user-search"
            type="text"
            placeholder="Search users..."
            className="pl-9 !rounded-[var(--radius-full)] !py-2 !text-sm !bg-[var(--color-surface-primary)]"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-1.5 py-1">
        {isBusy && (
          <div className="p-4 text-center text-xs text-[var(--color-text-muted)] animate-pulse-soft">
            {search ? 'Searching...' : 'Loading conversations...'}
          </div>
        )}

        {!isBusy && displayList.length === 0 && (
          <div className="p-8 text-center animate-fade-in">
            <div className="w-12 h-12 rounded-full bg-[var(--color-surface-tertiary)] flex items-center justify-center mx-auto mb-3">
              <Search size={20} className="text-[var(--color-text-muted)]" />
            </div>
            <p className="text-sm text-[var(--color-text-muted)]">
              {search ? 'No users found' : 'No conversations yet'}
            </p>
            {!search && (
              <p className="text-xs text-[var(--color-text-muted)] mt-1 opacity-60">
                Search for users to start chatting
              </p>
            )}
          </div>
        )}

        {!isBusy &&
          displayList.map((partner) => (
            <PartnerItem
              key={partner.id}
              partner={partner}
              isActive={activePartner?.id === partner.id}
              isMe={partner.id === user?.id}
              preview={search ? undefined : previews.get(partner.id)}
              onClick={() => handleSelectPartner(partner)}
            />
          ))}
      </div>
    </div>
  );
};
