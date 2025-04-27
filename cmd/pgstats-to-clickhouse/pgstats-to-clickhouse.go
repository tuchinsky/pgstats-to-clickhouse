package main

import (
	"context"
	"database/sql"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/tuchinsky/pgstats-to-clickhouse/internal"
)

var usage = `pgstats-to-clickhouse - collects pg_stat_statements, pg_stat_activity, pg_statio_all_tables and pg_stat_tables output and pushes to remote clickhouse

read settings from ENV:
	INTERVAL - collect interval in seconds (default: "30s")
	DISCOVERY_INTERVAL - interval for discovering postgres databases (default: "30s")
	POSTGRES_DSN - connection to postgres for pg_stat_statements and pg_stat_activity (default: "postgres://postgres@localhost:5432/postgres?sslmode=disable")
	CLICKHOUSE_DSN - connection to clickhouse (default: "http://localhost:8123/default")
`

func main() {
	cfg, err := internal.NewConfig()
	if err != nil {
		log.Println(usage)
		log.Fatalf(err.Error())
	}

	log.Println("- - - - - - - - - - - - - - -")
	log.Println("daemon started")

	var wg sync.WaitGroup
	wg.Add(1)
	go runDiscovery(handleSignals(), cfg.DiscoveryInterval, cfg.Interval, cfg.PostgresDsn, cfg.ClickhouseDsn, &wg)

	wg.Add(1)
	go setupPSSCollector(handleSignals(), cfg.Interval, cfg.PostgresDsn, cfg.ClickhouseDsn, &wg)

	wg.Add(1)
	go setupPSACollector(handleSignals(), cfg.Interval, cfg.PostgresDsn, cfg.ClickhouseDsn, &wg)

	wg.Wait()

	log.Println("daemon terminated")
}

func handleSignals() context.Context {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		oscall := <-c
		log.Printf("got system call:%+v", oscall)
		cancel()
	}()

	return ctx
}

func setupPSSCollector(ctx context.Context, interval time.Duration, postgresDsn string, clickhouseDsn string, wg *sync.WaitGroup) {
	defer wg.Done()
	setupCollector(ctx, &internal.PgStatStatementsFactory{}, interval, postgresDsn, clickhouseDsn)
}

func setupPSACollector(ctx context.Context, interval time.Duration, postgresDsn string, clickhouseDsn string, wg *sync.WaitGroup) {
	defer wg.Done()
	setupCollector(ctx, &internal.PgStatActivityFactory{}, interval, postgresDsn, clickhouseDsn)
}

func setupPSTCollector(ctx context.Context, interval time.Duration, postgresDsn string, clickhouseDsn string, wg *sync.WaitGroup) {
	defer wg.Done()
	setupCollector(ctx, &internal.PgStatioTableFactory{}, interval, postgresDsn, clickhouseDsn)
}

func setupPTSCollector(ctx context.Context, interval time.Duration, postgresDsn string, clickhouseDsn string, wg *sync.WaitGroup) {
	defer wg.Done()
	setupCollector(ctx, &internal.PgTableSizeFactory{}, interval, postgresDsn, clickhouseDsn)
}

func setupCollector(ctx context.Context, collector internal.CollectorFactory, interval time.Duration, postgresDsn string, clickhouseDsn string) {
	hostname, _ := os.Hostname()
	ttl := int64(interval/time.Second) * 2
	sc, err := internal.NewStatsCollector(
		collector,
		hostname,
		postgresDsn,
		clickhouseDsn,
		ttl,
	)
	if err != nil {
		log.Fatalf("[%s] Unable to init collector: %v", collector.Name(), err)
	}

	u, err := url.Parse(postgresDsn)
	if err != nil {
		log.Printf("[%s] Failed to parse postgres DSN: %v", collector.Name(), err)
		return
	}
	dbName := "unknown"
	if u.Path != "" && len(u.Path) > 1 {
		dbName = u.Path[1:]
	}

	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := sc.Tick(); err != nil {
					log.Printf("[%s] Error during tick: %v", collector.Name(), err)
				}
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	log.Printf("[%s] collector started for database %s", collector.Name(), dbName)

	<-ctx.Done()

	if err = sc.Shutdown(); err != nil {
		log.Fatalf("[%s] collector shutdown failed for database %s: %v", collector.Name(), dbName, err)
	}

	log.Printf("[%s] collector stopped for database %s", collector.Name(), dbName)
}

func runDiscovery(ctx context.Context, DiscoveryInterval time.Duration, collectionInterval time.Duration, baseDSN string, clickhouseDSN string, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(DiscoveryInterval)
	defer ticker.Stop()

	collectors := make(map[string]context.CancelFunc)

	for {
		select {
		case <-ticker.C:
			databases, err := getDatabases(ctx, baseDSN)
			if err != nil {
				log.Printf("Error getting databases: %v", err)
				continue
			}

			currentDBs := make(map[string]bool)
			for _, db := range databases {
				currentDBs[db] = true
				if _, exists := collectors[db]; !exists {
					collectorCtx, cancel := context.WithCancel(ctx)
					collectors[db] = cancel
					wg.Add(1)
					go setupPSTCollector(collectorCtx, collectionInterval, replaceDatabaseInDSN(baseDSN, db), clickhouseDSN, wg)
					wg.Add(1)
					go setupPTSCollector(collectorCtx, collectionInterval, replaceDatabaseInDSN(baseDSN, db), clickhouseDSN, wg)
				}
			}

			for db, cancel := range collectors {
				if !currentDBs[db] {
					cancel()
					delete(collectors, db)
				}
			}
		case <-ctx.Done():
			for _, cancel := range collectors {
				cancel()
			}
			return
		}
	}
}

func getDatabases(ctx context.Context, baseDSN string) ([]string, error) {
	db, err := sql.Open("postgres", baseDSN)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SELECT datname FROM pg_database WHERE datname NOT IN ('template0', 'template1', 'postgres')")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var datname string
		if err := rows.Scan(&datname); err != nil {
			return nil, err
		}
		databases = append(databases, datname)
	}
	return databases, nil
}

func replaceDatabaseInDSN(dsn string, newDB string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		log.Fatalf("Invalid DSN: %v", err)
	}
	u.Path = "/" + newDB
	return u.String()
}
