package stores

import (
	"database/sql"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type mysqlLatencyInput struct {
	DSN         string
	Query       string
	SampleCount int
}

type latencyComparisonOutput struct {
	MySQLMedian time.Duration
	GORMMedian  time.Duration
}

func TestCompareMySQLCLIAndGORMSelectLatency(t *testing.T) {
	if strings.TrimSpace(os.Getenv("RUN_DB_LATENCY_TEST")) != "1" {
		t.Skip("set RUN_DB_LATENCY_TEST=1 to run DB latency comparison")
	}

	rawDSN := strings.TrimSpace(os.Getenv("DB_DSN"))
	if rawDSN == "" {
		t.Fatal("DB_DSN is required")
	}

	mysqlPath, err := exec.LookPath("mysql")
	if err != nil {
		t.Skip("mysql client binary not found")
	}

	query := defaultString(strings.TrimSpace(os.Getenv("DB_LATENCY_QUERY")), "SELECT * FROM posts")
	sampleCount := defaultInt(strings.TrimSpace(os.Getenv("DB_LATENCY_SAMPLES")), 5)

	comparisonOutput, err := compareMySQLAndGORM(mysqlLatencyInput{
		DSN:         rawDSN,
		Query:       query,
		SampleCount: sampleCount,
	}, mysqlPath)
	if err != nil {
		t.Fatalf("latency compare failed: %v", err)
	}

	ratio := float64(comparisonOutput.GORMMedian) / float64(comparisonOutput.MySQLMedian)
	t.Logf("mysql median=%s gorm median=%s ratio=%.2f", comparisonOutput.MySQLMedian, comparisonOutput.GORMMedian, ratio)

	maxRatio := defaultFloat(strings.TrimSpace(os.Getenv("DB_LATENCY_MAX_RATIO")), 2.0)
	if ratio > maxRatio {
		t.Fatalf("gorm latency ratio too high: got %.2f want <= %.2f", ratio, maxRatio)
	}
}

func compareMySQLAndGORM(input mysqlLatencyInput, mysqlPath string) (latencyComparisonOutput, error) {
	mysqlDurations, err := measureMySQLCLIDurations(input, mysqlPath)
	if err != nil {
		return latencyComparisonOutput{}, err
	}

	gormDurations, err := measureGORMDurations(input)
	if err != nil {
		return latencyComparisonOutput{}, err
	}

	return latencyComparisonOutput{
		MySQLMedian: medianDuration(mysqlDurations),
		GORMMedian:  medianDuration(gormDurations),
	}, nil
}

func measureMySQLCLIDurations(input mysqlLatencyInput, mysqlPath string) ([]time.Duration, error) {
	parsedDSN, err := driver.ParseDSN(normalizeDSN(input.DSN))
	if err != nil {
		return nil, err
	}

	host, port, err := splitHostPort(parsedDSN.Addr)
	if err != nil {
		return nil, err
	}

	measurements := make([]time.Duration, 0, input.SampleCount)
	for i := 0; i < input.SampleCount; i++ {
		measurement, err := runMySQLCommand(mysqlCommandInput{
			MySQLPath: mysqlPath,
			User:      parsedDSN.User,
			Password:  parsedDSN.Passwd,
			Host:      host,
			Port:      port,
			Database:  parsedDSN.DBName,
			Query:     input.Query,
		})
		if err != nil {
			return nil, err
		}
		measurements = append(measurements, measurement)
	}

	return measurements, nil
}

func measureGORMDurations(input mysqlLatencyInput) ([]time.Duration, error) {
	db, err := gorm.Open(gormmysql.Open(normalizeDSN(input.DSN)), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = sqlDB.Close()
	}()

	measurements := make([]time.Duration, 0, input.SampleCount)
	for i := 0; i < input.SampleCount; i++ {
		startedAt := time.Now()
		rows, err := db.Raw(input.Query).Rows()
		if err != nil {
			return nil, err
		}
		if err := consumeRows(rows); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		measurements = append(measurements, time.Since(startedAt))
	}

	return measurements, nil
}

type mysqlCommandInput struct {
	MySQLPath string
	User      string
	Password  string
	Host      string
	Port      string
	Database  string
	Query     string
}

func runMySQLCommand(input mysqlCommandInput) (time.Duration, error) {
	commandArgs := []string{
		"-u", input.User,
		"-h", input.Host,
		"-P", input.Port,
		"-D", input.Database,
		"-N",
		"-s",
		"-e", input.Query,
	}

	command := exec.Command(input.MySQLPath, commandArgs...)
	command.Env = append(os.Environ(), "MYSQL_PWD="+input.Password)
	command.Stdout = io.Discard
	command.Stderr = io.Discard

	startedAt := time.Now()
	if err := command.Run(); err != nil {
		return 0, err
	}

	return time.Since(startedAt), nil
}

func consumeRows(rows *sql.Rows) error {
	columnNames, err := rows.Columns()
	if err != nil {
		return err
	}

	rawValues := make([]sql.RawBytes, len(columnNames))
	scanTargets := make([]any, len(columnNames))
	for index := range rawValues {
		scanTargets[index] = &rawValues[index]
	}

	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return err
		}
	}

	return rows.Err()
}

func splitHostPort(address string) (string, string, error) {
	lastColonIndex := strings.LastIndex(address, ":")
	if lastColonIndex <= 0 || lastColonIndex == len(address)-1 {
		return "", "", fmt.Errorf("invalid mysql address %q", address)
	}

	host := address[:lastColonIndex]
	port := address[lastColonIndex+1:]
	return host, port, nil
}

func medianDuration(values []time.Duration) time.Duration {
	sortedValues := make([]time.Duration, len(values))
	copy(sortedValues, values)
	sort.Slice(sortedValues, func(i, j int) bool {
		return sortedValues[i] < sortedValues[j]
	})

	middleIndex := len(sortedValues) / 2
	if len(sortedValues)%2 == 0 {
		sum := sortedValues[middleIndex-1] + sortedValues[middleIndex]
		return sum / 2
	}
	return sortedValues[middleIndex]
}

func defaultInt(rawValue string, fallback int) int {
	if rawValue == "" {
		return fallback
	}
	parsedValue, err := strconv.Atoi(rawValue)
	if err != nil || parsedValue <= 0 {
		return fallback
	}
	return parsedValue
}

func defaultFloat(rawValue string, fallback float64) float64 {
	if rawValue == "" {
		return fallback
	}

	parsedValue, err := strconv.ParseFloat(rawValue, 64)
	if err != nil || parsedValue <= 0 || math.IsNaN(parsedValue) {
		return fallback
	}
	return parsedValue
}

func defaultString(rawValue string, fallback string) string {
	if rawValue == "" {
		return fallback
	}
	return rawValue
}
