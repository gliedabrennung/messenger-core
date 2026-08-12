package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gliedabrennung/sedna/internal/common/logger"
	"github.com/gliedabrennung/sedna/internal/config"
	"github.com/gliedabrennung/sedna/internal/controller/http"
	"github.com/gliedabrennung/sedna/internal/controller/http/middleware"
	"github.com/gliedabrennung/sedna/internal/domain"
	"github.com/gliedabrennung/sedna/internal/fanout"
	"github.com/gliedabrennung/sedna/internal/repository/message"
	"github.com/gliedabrennung/sedna/internal/repository/postgres"
	"github.com/gliedabrennung/sedna/internal/repository/tokens"
	"github.com/gliedabrennung/sedna/internal/usecase"
	"github.com/gliedabrennung/sedna/internal/ws"
	"github.com/gliedabrennung/sedna/migrations"
	"github.com/gocql/gocql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		logger.Fatalf("startup failed: %v", err)
	}
}

func run() error {
	var (
		msgRepo       domain.MessageRepository
		scyllaSession *gocql.Session
	)

	cfg, err := config.LoadConfig(".env")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	trustedProxies, err := cfg.ParseTrustedProxies()
	if err != nil {
		return err
	}

	ctx := context.Background()

	dbpool, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return fmt.Errorf("create connection pool: %w", err)
	}
	defer dbpool.Close()

	if cfg.RunMigrations {
		if err := postgres.Migrate(ctx, dbpool, migrations.SQL); err != nil {
			return err
		}
	}

	degrade := func(what string, err error) error {
		if cfg.StorageRequired {
			return fmt.Errorf("%s unavailable (set STORAGE_REQUIRED=false to start anyway): %w", what, err)
		}
		logger.Warnf("warning: %s unavailable, continuing degraded: %v", what, err)
		return nil
	}

	if err := message.InitSchema(ctx, cfg.ScyllaHosts, cfg.ScyllaKeyspace); err != nil {
		if err := degrade("scylla schema", err); err != nil {
			return err
		}
	} else {
		cluster := gocql.NewCluster(cfg.ScyllaHosts...)
		cluster.Keyspace = cfg.ScyllaKeyspace
		cluster.Timeout = 5 * time.Second
		var err error
		scyllaSession, err = cluster.CreateSession()
		if err != nil {
			scyllaSession = nil
			if err := degrade("scylla", err); err != nil {
				return err
			}
		} else {
			defer scyllaSession.Close()
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		if closeErr := rdb.Close(); closeErr != nil {
			logger.Warnf("warning: could not close redis connection: %v", closeErr)
		}
		rdb = nil
		if err := degrade("redis", err); err != nil {
			return err
		}
	} else {
		defer func() {
			if err := rdb.Close(); err != nil {
				logger.Warnf("warning: could not close redis connection: %v", err)
			}
		}()
	}

	if scyllaSession != nil {
		msgRepo = message.NewRepository(scyllaSession, rdb, cfg.ScyllaKeyspace)
		if rdb == nil {
			logger.Warn("redis unavailable: message history served without cache")
		}
	} else {
		logger.Warn("scylla unavailable: messages will not be stored")
	}

	repo := postgres.NewPostgresRepository(dbpool)
	authUseCase := usecase.NewAuthUseCase(repo, cfg.JWTSecret, cfg.JWTTTL)

	var revoked middleware.RevocationChecker
	if rdb != nil {
		denylist := tokens.NewRedisDenylist(rdb)
		authUseCase.SetDenylist(denylist)
		revoked = denylist
	} else {
		logger.Warn("redis unavailable: logout cannot revoke issued tokens")
	}

	userUseCase := usecase.NewUserUseCase(repo)

	hubCtx, hubCancel := context.WithCancel(ctx)
	defer hubCancel()

	var msgFanout ws.Fanout
	if rdb != nil {
		redisFanout := fanout.NewRedis(hubCtx, rdb)
		defer func() {
			if err := redisFanout.Close(); err != nil {
				logger.Warnf("warning: could not close fanout: %v", err)
			}
		}()
		msgFanout = redisFanout
	} else {
		logger.Warn("redis unavailable: websocket delivery limited to this instance")
	}

	messageUseCase := usecase.NewMessageUseCase(msgRepo)
	messageUseCase.SetUsers(repo)

	hub := ws.NewHubWithFanout(msgRepo, msgFanout)
	hub.SetRecipientValidator(repo)
	go hub.Run(hubCtx)

	h := server.Default(
		server.WithHostPorts(cfg.Addr),
		server.WithHandleMethodNotAllowed(true),
	)

	h.SetClientIPFunc(app.ClientIPWithOption(app.ClientIPOptions{
		RemoteIPHeaders: []string{"X-Forwarded-For", "X-Real-IP"},
		TrustedCIDRs:    trustedProxies,
	}))
	if len(trustedProxies) == 0 {
		logger.Info("TRUSTED_PROXIES empty: forwarded-for headers ignored for client IP")
	}

	h.OnShutdown = append(h.OnShutdown, func(ctx context.Context) {
		hub.Stop()
		hubCancel()
	})

	upgrader := ws.NewUpgrader(cfg.AllowedOrigins)
	wsHandler := ws.ServeWs(hub, upgrader)

	cookieCfg := http.CookieConfig{
		Name:   "token",
		MaxAge: int(cfg.JWTTTL.Seconds()),
		Secure: cfg.CookieSecure,
		Domain: cfg.CookieDomain,
	}
	if !cfg.CookieSecure {
		logger.Warn("COOKIE_SECURE is false: the auth cookie will be sent over plain HTTP")
	}

	http.SetupRouter(h, http.Deps{
		Auth:      authUseCase,
		Users:     userUseCase,
		Messages:  messageUseCase,
		WsHandler: wsHandler,
		JWTSecret: cfg.JWTSecret,
		Cookie:    cookieCfg,
		Revoked:   revoked,
	})

	h.Spin()
	return nil
}
