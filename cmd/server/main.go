package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jdk829355/InForest_back/config"
	app "github.com/jdk829355/InForest_back/internal/grpc/forestservice"
	"github.com/jdk829355/InForest_back/internal/store"
	gen "github.com/jdk829355/InForest_back/protos/forest"
	"google.golang.org/grpc"
)

func main() {
	// configuration loading
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	// database connection
	driver, err := store.InitNeo4jStore(cfg)
	if err != nil {
		panic(err)
	}
	store := store.NewStore(*driver)
	ctx := context.Background()
	defer store.Close(ctx)

	// gRPC server setup
	forestService := app.NewForestService(store)
	l, e := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPC_PORT))
	if e != nil {
		panic(e)
	}
	s := grpc.NewServer()

	gen.RegisterForestServiceServer(s, forestService)

	go func() {
		log.Printf("Starting gRPC server on :%s", cfg.GRPC_PORT)
		if err := s.Serve(l); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 시그널이 수신될 때까지 대기
	<-quit
	log.Println("Shutting down gRPC server...")

	// gRPC 서버의 우아한 종료 (진행 중인 요청 완료 대기)
	s.GracefulStop()
	log.Println("gRPC server stopped gracefully.")

	// (참고) DB 연결 닫기는 main 함수의 defer가 처리
}
