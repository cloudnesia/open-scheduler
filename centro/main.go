package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	centrogrpc "github.com/open-scheduler/centro/grpc"
	"github.com/open-scheduler/centro/migration"
	"github.com/open-scheduler/centro/scheduler"
	"github.com/open-scheduler/centro/rest"
	etcdstorage "github.com/open-scheduler/centro/storage/etcd"
	pb "github.com/open-scheduler/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	_ "github.com/open-scheduler/docs" // This line is needed for Swagger
)

// @title Open Scheduler API
// @version 1.0
// @description A distributed job scheduler REST API for managing jobs and nodes
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.open-scheduler.io/support
// @contact.email support@open-scheduler.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your JWT token in the format: Bearer {token}

func main() {
	port := flag.String("port", "50051", "The gRPC server port")
	httpPort := flag.String("http-port", "8080", "The REST API server port")
	etcdEndpoints := flag.String("etcd-endpoints", "localhost:2379", "Comma-separated list of etcd endpoints")
	flag.Parse()

	endpoints := strings.Split(*etcdEndpoints, ",")
	for i := range endpoints {
		endpoints[i] = strings.TrimSpace(endpoints[i])
	}

	log.Printf("[Centro] Connecting to etcd endpoints: %v", endpoints)
	storage, err := etcdstorage.NewStorage(endpoints)
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}
	defer storage.Close()

	log.Printf("[Centro] Successfully connected to etcd")

	address := fmt.Sprintf(":%s", *port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", address, err)
	}

	log.Printf("[Centro] Starting gRPC server on %s", address)

	// Load TLS configuration for gRPC server
	tlsConfig := centrogrpc.LoadServerTLSConfigFromEnv()
	var grpcServer *grpc.Server

	if tlsConfig.Enabled {
		creds, err := centrogrpc.GetServerTLSCredentials(tlsConfig)
		if err != nil {
			log.Fatalf("Failed to setup TLS credentials: %v", err)
		}
		grpcServer = grpc.NewServer(grpc.Creds(creds))
		log.Printf("[Centro] TLS enabled for gRPC server")
	} else {
		grpcServer = grpc.NewServer()
		log.Printf("[Centro] WARNING: TLS is disabled. Enable GRPC_SERVER_TLS_ENABLED in production!")
	}

	centroServer := centrogrpc.NewCentroServer(storage)
	pb.RegisterCentroSchedulerServiceServer(grpcServer, centroServer)

	reflection.Register(grpcServer)

	apiServer := rest.NewAPIServer(storage)
	httpAddress := fmt.Sprintf(":%s", *httpPort)
	httpServer := &http.Server{
		Addr:    httpAddress,
		Handler: apiServer.GetRouter(),
	}

	// Create cancellable context for all background goroutines
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	queue := scheduler.NewQueue(storage)
	go queue.StartScheduler(bgCtx)

	go func() {
		time.Sleep(5 * time.Second)
		migration.SeedTestData(centroServer)
	}()

	// Status logging goroutine with proper cancellation
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-bgCtx.Done():
				log.Printf("[Centro] Status logging goroutine shutting down")
				return
			case <-ticker.C:
				nodeCount := centroServer.GetNodeCount()
				queued, active, completed := centroServer.GetJobStats()
				log.Printf("[Centro] Status - Nodes: %d, Jobs (Queued: %d, Active: %d, Completed: %d)",
					nodeCount, queued, active, completed)
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	go func() {
		log.Printf("[Centro] Starting REST API server on %s", httpAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to serve REST API: %v", err)
		}
	}()

	log.Printf("[Centro] gRPC server is ready and listening on %s", address)
	log.Printf("[Centro] REST API server is ready and listening on %s", httpAddress)
	log.Println("[Centro] Press Ctrl+C to stop")

	<-sigChan
	log.Println("\n[Centro] Shutting down gracefully...")

	// Cancel all background goroutines first
	bgCancel()
	log.Printf("[Centro] Stopping background services...")
	// Give goroutines a moment to exit cleanly
	time.Sleep(100 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Centro] HTTP server shutdown error: %v", err)
	}

	grpcServer.GracefulStop()
	log.Println("[Centro] Servers stopped")
}
