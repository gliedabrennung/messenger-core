import { memo, type FC } from 'react';
import clsx from 'clsx';
import { Avatar } from '@/components/ui/Avatar';
import type { User } from '@/types';

interface PartnerItemProps {
  partner: User;
  isActive: boolean;
  isMe?: boolean;
  preview?: string;
  onClick: () => void;
}

const PartnerItemBase: FC<PartnerItemProps> = ({ partner, isActive, isMe, preview, onClick }) => {
  return (
    <button
      id={`partner-${partner.id}`}
      onClick={onClick}
      className={clsx(
        'w-full p-3.5 flex items-center gap-3 transition-all duration-200 text-left',
        'hover:bg-[var(--color-surface-tertiary)]/40 rounded-[var(--radius-md)] mx-1.5',
        isActive && 'bg-[var(--color-accent-glow)] border-l-2 border-[var(--color-accent-start)]'
      )}
    >
      <Avatar username={partner.username} />
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium text-[var(--color-text-primary)] truncate">
          {partner.username}
          {isMe && (
            <span className="ml-1.5 text-xs font-normal text-[var(--color-accent-start)]">(you)</span>
          )}
        </div>
        <div className="text-xs text-[var(--color-text-muted)] truncate mt-0.5">
          {preview || `ID: ${partner.id}`}
        </div>
      </div>
    </button>
  );
};

export const PartnerItem = memo(PartnerItemBase);
