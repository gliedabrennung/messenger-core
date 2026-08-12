package message

import (
	"context"
	"fmt"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/gliedabrennung/sedna/internal/common/logger"
	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/gocql/gocql"
)

type ScyllaStorage struct {
	session  *gocql.Session
	keyspace string
}

func InitSchema(ctx context.Context, hosts []string, keyspace string) error {
	cluster := gocql.NewCluster(hosts...)
	cluster.Timeout = 5 * time.Second
	session, err := cluster.CreateSession()
	if err != nil {
		logger.CtxErrorf(ctx, "failed to connect to scylla cluster for schema init: %v", err)
		return fmt.Errorf("scylla schema init: connect cluster: %w", err)
	}
	defer session.Close()

	createKeyspaceQuery := fmt.Sprintf(`
		CREATE KEYSPACE IF NOT EXISTS %s
		WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`, keyspace)
	if err := session.Query(createKeyspaceQuery).WithContext(ctx).Exec(); err != nil {
		logger.CtxErrorf(ctx, "failed to create keyspace %s: %v", keyspace, err)
		return fmt.Errorf("scylla schema init: create keyspace: %w", err)
	}

	createTableQuery := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.direct_messages (
			chat_id text,
			message_id timeuuid,
			from_id bigint,
			to_id bigint,
			content text,
			created_at timestamp,
			PRIMARY KEY ((chat_id), message_id)
		) WITH CLUSTERING ORDER BY (message_id DESC)
			AND compaction = {'class': 'TimeWindowCompactionStrategy',
			                  'compaction_window_unit': 'DAYS',
			                  'compaction_window_size': 7}
			AND gc_grace_seconds = 864000`, keyspace)
	if err := session.Query(createTableQuery).WithContext(ctx).Exec(); err != nil {
		logger.CtxErrorf(ctx, "failed to create direct_messages table in %s: %v", keyspace, err)
		return fmt.Errorf("scylla schema init: create table: %w", err)
	}

	createChatsQuery := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.user_chats (
			user_id bigint,
			peer_id bigint,
			chat_id text,
			last_message text,
			last_from_id bigint,
			last_activity timestamp,
			PRIMARY KEY ((user_id), peer_id)
		)`, keyspace)
	if err := session.Query(createChatsQuery).WithContext(ctx).Exec(); err != nil {
		logger.CtxErrorf(ctx, "failed to create user_chats table in %s: %v", keyspace, err)
		return fmt.Errorf("scylla schema init: create user_chats: %w", err)
	}

	logger.CtxInfof(ctx, "scylla schema initialized successfully in keyspace %s", keyspace)
	return nil
}

func NewScyllaStorage(session *gocql.Session, keyspace string) *ScyllaStorage {
	return &ScyllaStorage{session: session, keyspace: keyspace}
}

func (s *ScyllaStorage) Save(ctx context.Context, msg *entity.Message) error {
	var id gocql.UUID
	if msg.MessageID == "" {
		id = gocql.TimeUUID()
		msg.MessageID = id.String()
	} else {
		parsed, err := gocql.ParseUUID(msg.MessageID)
		if err != nil {
			return fmt.Errorf("scylla: invalid message id %q: %w", msg.MessageID, err)
		}
		id = parsed
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.direct_messages
		(chat_id, message_id, from_id, to_id, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, s.keyspace)
	err := s.session.Query(query,
		msg.ChatID, id, msg.FromID, msg.ToID, msg.Content, msg.CreatedAt,
	).WithContext(ctx).Exec()

	if err != nil {
		logger.CtxErrorf(ctx, "scylla save failed for chat %s: %v", msg.ChatID, err)
		return fmt.Errorf("scylla: save message: %w", err)
	}

	logger.CtxDebugf(ctx, "scylla saved message %s for chat %s", msg.MessageID, msg.ChatID)
	return nil
}

func (s *ScyllaStorage) GetHistory(ctx context.Context, chatID string, limit int, cursor string) ([]*entity.Message, string, error) {
	var query *gocql.Query

	if cursor == "" {
		query = s.session.Query(fmt.Sprintf(`
			SELECT chat_id, message_id, from_id, to_id, content, created_at
			FROM %s.direct_messages
			WHERE chat_id = ?
			ORDER BY message_id DESC
			LIMIT ?`, s.keyspace), chatID, limit+1,
		).WithContext(ctx)
	} else {
		cursorUUID, err := gocql.ParseUUID(cursor)
		if err != nil {
			logger.CtxErrorf(ctx, "scylla parse cursor %s failed: %v", cursor, err)
			return nil, "", fmt.Errorf("scylla: parse cursor: %w", err)
		}
		query = s.session.Query(fmt.Sprintf(`
			SELECT chat_id, message_id, from_id, to_id, content, created_at
			FROM %s.direct_messages
			WHERE chat_id = ? AND message_id < ?
			ORDER BY message_id DESC
			LIMIT ?`, s.keyspace), chatID, cursorUUID, limit+1,
		).WithContext(ctx)
	}

	iter := query.Iter()
	messages := make([]*entity.Message, 0, limit+1)

	var (
		msgChatID string
		msgID     gocql.UUID
		fromID    int64
		toID      int64
		content   string
		createdAt time.Time
	)

	for iter.Scan(&msgChatID, &msgID, &fromID, &toID, &content, &createdAt) {
		messages = append(messages, &entity.Message{
			ChatID:    msgChatID,
			MessageID: msgID.String(),
			FromID:    fromID,
			ToID:      toID,
			Content:   content,
			CreatedAt: createdAt,
		})
	}

	if err := iter.Close(); err != nil {
		logger.CtxErrorf(ctx, "scylla get history failed for chat %s: %v", chatID, err)
		return nil, "", fmt.Errorf("scylla: get chat history: %w", err)
	}

	var nextCursor string
	if len(messages) > limit {
		nextCursor = messages[limit-1].MessageID
		messages = messages[:limit]
	}

	logger.CtxDebugf(ctx, "scylla retrieved %d messages for chat %s", len(messages), chatID)
	return messages, nextCursor, nil
}

const maxChatPreviewLen = 200

func (s *ScyllaStorage) TouchChats(ctx context.Context, msg *entity.Message) error {
	preview := msg.Content
	if utf8.RuneCountInString(preview) > maxChatPreviewLen {
		preview = string([]rune(preview)[:maxChatPreviewLen]) + "…"
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.user_chats
		(user_id, peer_id, chat_id, last_message, last_from_id, last_activity)
		VALUES (?, ?, ?, ?, ?, ?)
		USING TIMESTAMP ?`, s.keyspace)

	writeTime := msg.CreatedAt.UnixMicro()

	sides := [2]struct{ owner, peer int64 }{
		{msg.FromID, msg.ToID},
		{msg.ToID, msg.FromID},
	}

	for _, side := range sides {
		err := s.session.Query(query,
			side.owner, side.peer, msg.ChatID, preview, msg.FromID, msg.CreatedAt, writeTime,
		).WithContext(ctx).Exec()
		if err != nil {
			return fmt.Errorf("scylla: touch chat for user %d: %w", side.owner, err)
		}
	}
	return nil
}

func (s *ScyllaStorage) ListChats(ctx context.Context, userID int64, limit int) ([]*entity.Chat, error) {
	query := fmt.Sprintf(`
		SELECT peer_id, chat_id, last_message, last_from_id, last_activity
		FROM %s.user_chats
		WHERE user_id = ?`, s.keyspace)

	iter := s.session.Query(query, userID).WithContext(ctx).Iter()

	var (
		chats        []*entity.Chat
		peerID       int64
		chatID       string
		lastMessage  string
		lastFromID   int64
		lastActivity time.Time
	)

	for iter.Scan(&peerID, &chatID, &lastMessage, &lastFromID, &lastActivity) {
		chats = append(chats, &entity.Chat{
			ChatID:       chatID,
			PeerID:       peerID,
			LastMessage:  lastMessage,
			LastFromID:   lastFromID,
			LastActivity: lastActivity,
		})
	}

	if err := iter.Close(); err != nil {
		logger.CtxErrorf(ctx, "scylla list chats for user %d: %v", userID, err)
		return nil, fmt.Errorf("scylla: list chats: %w", err)
	}

	sort.Slice(chats, func(i, j int) bool {
		return chats[i].LastActivity.After(chats[j].LastActivity)
	})
	if limit > 0 && len(chats) > limit {
		chats = chats[:limit]
	}
	return chats, nil
}
