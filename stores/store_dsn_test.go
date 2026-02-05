package stores

import (
	"testing"

	driver "github.com/go-sql-driver/mysql"
)

func TestNormalizeDSNAddsPerformanceDefaults(t *testing.T) {
	inputDSN := "user:pass@tcp(localhost:3306)/explore?charset=utf8mb4"

	normalizedDSN := normalizeDSN(inputDSN)

	parsedDSN, err := driver.ParseDSN(normalizedDSN)
	if err != nil {
		t.Fatalf("expected parsable dsn: %v", err)
	}

	if !parsedDSN.ParseTime {
		t.Fatal("expected parseTime=true")
	}
	if !parsedDSN.InterpolateParams {
		t.Fatal("expected interpolateParams=true")
	}
	if parsedDSN.TLSConfig != "skip-verify" {
		t.Fatalf("expected tls=skip-verify, got %q", parsedDSN.TLSConfig)
	}
}

func TestNormalizeDSNRespectsExplicitTLSPolicy(t *testing.T) {
	inputDSN := "user:pass@tcp(localhost:3306)/explore?charset=utf8mb4&tls=preferred"

	normalizedDSN := normalizeDSN(inputDSN)

	parsedDSN, err := driver.ParseDSN(normalizedDSN)
	if err != nil {
		t.Fatalf("expected parsable dsn: %v", err)
	}

	if parsedDSN.TLSConfig != "preferred" {
		t.Fatalf("expected tls=preferred, got %q", parsedDSN.TLSConfig)
	}
	if !parsedDSN.InterpolateParams {
		t.Fatal("expected interpolateParams=true")
	}
}
