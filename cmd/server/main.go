package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jdk829355/InForest_back/config"
	app "github.com/jdk829355/InForest_back/internal/grpc/forestservice"
	"github.com/jdk829355/InForest_back/internal/store"
	gen "github.com/jdk829355/InForest_back/protos/forest"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

func main() {
	// Zap 로거 초기화
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("failed to initialize zap logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() // 애플리케이션 종료 전 버퍼 로그 플러시

	// 설정 로드
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}
	logger.Info("Configuration loaded successfully")

	// Neo4j 스토어 초기화
	driver, err := store.InitNeo4jStore(cfg)
	if err != nil {
		logger.Fatal("Failed to connect to Neo4j", zap.Error(err))
	}
	store := store.NewStore(*driver)
	ctx := context.Background()

	defer func() {
		logger.Info("Closing database connection...")
		if err := store.Neo4j.Close(ctx); err != nil {
			logger.Error("Failed to close database connection", zap.Error(err))
		}
	}()
	logger.Info("Database connection established")

	// gRPC 서버 및 ForestService 초기화
	forestService := app.NewForestService(store)

	listenAddr := fmt.Sprintf(":%s", cfg.GRPC_PORT)
	l, e := net.Listen("tcp", listenAddr)
	if e != nil {
		logger.Fatal("Failed to listen on address",
			zap.String("address", listenAddr),
			zap.Error(e),
		)
	}

	// 로거 옵션 설정
	loggingOpts := []grpc_zap.Option{
		grpc_zap.WithDurationField(func(duration time.Duration) zapcore.Field {
			return zap.Int64("grpc.time_ms", duration.Milliseconds())
		}),
	}

	// gRPC 서버 옵션에 인터셉터 추가
	serverOptions := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			// 패닉이 발생하면 복구하고 Internal 에러를 반환
			grpc_recovery.UnaryServerInterceptor(),
			// Zap을 사용해 요청/응답을 로깅
			grpc_zap.UnaryServerInterceptor(logger, loggingOpts...),
		)),
		// 스트림 사용할 경우에 대비해 추가
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_recovery.StreamServerInterceptor(),
			grpc_zap.StreamServerInterceptor(logger, loggingOpts...),
		)),
	}

	s := grpc.NewServer(serverOptions...) // 인터셉터 옵션 적용

	gen.RegisterForestServiceServer(s, forestService)

	// gRPC 서버 시작
	go func() {
		logger.Info("Starting gRPC server", zap.String("address", listenAddr))
		if err := s.Serve(l); err != nil {
			// Serve가 정상 종료(GracefulStop) 외의 이유로 중단되면 Fatal 로깅
			logger.Fatal("Failed to serve gRPC", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	// SIGINT, SIGTERM 시그널 수신 대기
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 시그널이 수신될 때까지 대기
	sig := <-quit
	logger.Info("Shutdown signal received", zap.String("signal", sig.String()))

	// gRPC 서버의 우아한 종료 (진행 중인 요청 완료 대기)
	s.GracefulStop()
	logger.Info("gRPC server stopped gracefully.")
}
