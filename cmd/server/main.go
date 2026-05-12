// cmd/server/main.go — точка входа HTTP-сервера.
//
// Что происходит при старте:
//   1. Загружаем data.json (или создаём пустой, если файла нет).
//   2. Поднимаем HTTP-сервер на :8080 с роутером из пакета api.
//   3. Ждём SIGINT/SIGTERM, делаем graceful shutdown и финальный save.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"plants-app/internal/api"
	"plants-app/internal/storage"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	store := storage.New("data.json")
	if err := store.Load(); err != nil {
		slog.Error("не удалось загрузить данные", "err", err)
		os.Exit(1)
	}

	// Порт можно задать через переменную окружения PORT.
	// Например: PORT=3001 go run cmd/server/main.go
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	addr := ":" + port
	url := "http://localhost:" + port

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewServer(store).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		fmt.Println("====================================")
		fmt.Println("  Растения — учёт запущен!")
		fmt.Println("  " + url)
		fmt.Println("  Для остановки: Ctrl+C")
		fmt.Println("====================================")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("сервер упал", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown — ждём SIGINT/SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("завершаем работу...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("ошибка остановки", "err", err)
	}
	if err := store.SaveNow(); err != nil {
		slog.Error("ошибка финального сохранения", "err", err)
	}
	slog.Info("готово. До свидания!")
}
