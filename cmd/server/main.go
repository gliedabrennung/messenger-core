package main

import (
	"context"
	"os"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/gliedabrennung/messenger-core/internal/config"
	"github.com/gliedabrennung/messenger-core/internal/controller/http"
	"github.com/gliedabrennung/messenger-core/internal/messenger"
	"github.com/gliedabrennung/messenger-core/internal/repository/postgres"
	"github.com/gliedabrennung/messenger-core/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
)

var addr = ":8080"

func main() {
	cfg := config.GetConfig()

	go messenger.StartHub()

	dbpool, err := pgxpool.New(context.Background(), cfg.DSN)
	if err != nil {
		hlog.Errorf("Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	if sqlBytes, err := os.ReadFile("migrations/0001.sql"); err == nil {
		if _, err := dbpool.Exec(context.Background(), string(sqlBytes)); err != nil {
			hlog.Errorf("Failed to run migrations: %v\n", err)
			os.Exit(1)
		}
	} else {
		hlog.Warnf("Could not read migration file: %v\n", err)
	}

	repo := postgres.NewPostgresRepository(dbpool)
	authUseCase := usecase.NewAuthUseCase(repo, cfg.JWTSecret, cfg.JWTTTL)

	h := server.Default(server.WithHostPorts(addr))
	server.WithHandleMethodNotAllowed(true)

	http.SetupRouter(h, authUseCase, cfg.JWTSecret)

	h.Spin()
}
