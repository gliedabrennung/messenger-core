CREATE KEYSPACE IF NOT EXISTS ws
    WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

CREATE TABLE IF NOT EXISTS ws.direct_messages (
    chat_id    TEXT,
    message_id TIMEUUID,
    from_id    BIGINT,
    to_id      BIGINT,
    content    TEXT,
    created_at TIMESTAMP,
    PRIMARY KEY ((chat_id), message_id)
) WITH CLUSTERING ORDER BY (message_id DESC)
    AND compaction = {'class': 'TimeWindowCompactionStrategy',
                      'compaction_window_unit': 'DAYS',
                      'compaction_window_size': 7}
    AND gc_grace_seconds = 864000;
