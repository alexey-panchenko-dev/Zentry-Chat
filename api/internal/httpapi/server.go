package httpapi

import (
	"context"
	"net/http"
	"newsChat/api/internal/webnotify"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"newsChat/api/internal/chat"
	"newsChat/api/internal/config"
	"newsChat/api/internal/realtime"
)

func NewServer(cfg config.Config, db *pgxpool.Pool, publisher realtime.Publisher) *http.Server {
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           boundedRequests(newRouter(db, publisher)),
		MaxHeaderBytes:    32 * 1024,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	go webnotify.New(db).Run(ctx)
	go chat.New(db).RunScheduled(ctx, publisher)
	server.RegisterOnShutdown(cancel)
	return server
}
