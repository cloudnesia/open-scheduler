package grpc

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc/credentials"
)

// ServerTLSConfig holds TLS configuration options for server
type ServerTLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
	CAFile   string
}

// GetServerTLSCredentials returns gRPC server credentials based on TLS configuration
func GetServerTLSCredentials(tlsConfig *ServerTLSConfig) (credentials.TransportCredentials, error) {
	if !tlsConfig.Enabled {
		log.Printf("[Centro gRPC] TLS is disabled, using insecure connection")
		return nil, nil
	}

	log.Printf("[Centro gRPC] Configuring TLS credentials")

	if tlsConfig.CertFile == "" || tlsConfig.KeyFile == "" {
		return nil, fmt.Errorf("TLS enabled but certificate or key file not specified")
	}

	cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}
	log.Printf("[Centro gRPC] Loaded server certificate from %s", tlsConfig.CertFile)

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Load CA certificate for mTLS if provided
	if tlsConfig.CAFile != "" {
		caCert, err := os.ReadFile(tlsConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := tls.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append certificates to pool")
		}
		tlsCfg.ClientCAs = caCertPool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		log.Printf("[Centro gRPC] Configured mTLS with CA certificate from %s", tlsConfig.CAFile)
	}

	return credentials.NewTLS(tlsCfg), nil
}

// LoadServerTLSConfigFromEnv loads server TLS configuration from environment variables
func LoadServerTLSConfigFromEnv() *ServerTLSConfig {
	return &ServerTLSConfig{
		Enabled:  os.Getenv("GRPC_SERVER_TLS_ENABLED") == "true",
		CertFile: os.Getenv("GRPC_SERVER_TLS_CERT_FILE"),
		KeyFile:  os.Getenv("GRPC_SERVER_TLS_KEY_FILE"),
		CAFile:   os.Getenv("GRPC_SERVER_TLS_CA_FILE"),
	}
}
