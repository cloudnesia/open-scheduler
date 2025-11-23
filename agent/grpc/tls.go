package grpc

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc/credentials"
)

// TLSConfig holds TLS configuration options
type TLSConfig struct {
	Enabled            bool
	CertFile           string
	KeyFile            string
	CAFile             string
	InsecureSkipVerify bool
}

// GetTLSCredentials returns gRPC credentials based on TLS configuration
func GetTLSCredentials(tlsConfig *TLSConfig) (credentials.TransportCredentials, error) {
	if !tlsConfig.Enabled {
		log.Printf("[GrpcClient] TLS is disabled, using insecure connection")
		return nil, nil
	}

	log.Printf("[GrpcClient] Configuring TLS credentials")

	// Load client certificate if provided
	if tlsConfig.CertFile != "" && tlsConfig.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		log.Printf("[GrpcClient] Loaded client certificate from %s", tlsConfig.CertFile)

		tlsCfg := &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
		}

		// Load CA certificate if provided
		if tlsConfig.CAFile != "" {
			caCert, err := os.ReadFile(tlsConfig.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA certificate: %w", err)
			}

			caCertPool, err := createCertPool(caCert)
			if err != nil {
				return nil, fmt.Errorf("failed to create CA cert pool: %w", err)
			}
			tlsCfg.RootCAs = caCertPool
			log.Printf("[GrpcClient] Loaded CA certificate from %s", tlsConfig.CAFile)
		}

		return credentials.NewTLS(tlsCfg), nil
	}

	// Server certificate only (no mTLS)
	if tlsConfig.CAFile != "" {
		caCert, err := os.ReadFile(tlsConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool, err := createCertPool(caCert)
		if err != nil {
			return nil, fmt.Errorf("failed to create CA cert pool: %w", err)
		}

		tlsCfg := &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
		}
		return credentials.NewTLS(tlsCfg), nil
	}

	// Default: use system CA certificates
	tlsCfg := &tls.Config{
		InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
	}
	return credentials.NewTLS(tlsCfg), nil
}

// createCertPool creates a certificate pool from PEM certificate data
func createCertPool(certPEM []byte) (*tls.CertPool, error) {
	certPool := tls.NewCertPool()
	if !certPool.AppendCertsFromPEM(certPEM) {
		return nil, fmt.Errorf("failed to append certificates to pool")
	}
	return certPool, nil
}

// LoadTLSConfigFromEnv loads TLS configuration from environment variables
func LoadTLSConfigFromEnv() *TLSConfig {
	return &TLSConfig{
		Enabled:            os.Getenv("GRPC_TLS_ENABLED") == "true",
		CertFile:           os.Getenv("GRPC_TLS_CERT_FILE"),
		KeyFile:            os.Getenv("GRPC_TLS_KEY_FILE"),
		CAFile:             os.Getenv("GRPC_TLS_CA_FILE"),
		InsecureSkipVerify: os.Getenv("GRPC_TLS_INSECURE_SKIP_VERIFY") == "true",
	}
}
