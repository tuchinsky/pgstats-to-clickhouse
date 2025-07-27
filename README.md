# pgstats-to-clickhouse

Collects data from PostgreSQL views and pushes it to remote Clickhouse:
 - pg_stat_statements
 - pg_stat_activity
 - pg_statio_user_tables
 - pg_stat_user_tables

Features:
- autodiscovery databases (see `runDiscovery` function) for collect pg_statio_user_tables and pg_stat_user_tables
- counts delta (counter -> gauge)
- skips not changing metrics (not for gauge metrics like n_live_tup, n_dead_tup, relation_size, etc)

## Example dashboards
![pg_stat_statements](examples/img/2e640f2055.png)

![pg_stat_tables](examples/img/62add3afdb.png)

## Usage
```bash
env INTERVAL="30s" \
    DISCOVERY_INTERVAL="30s" \
    POSTGRES_DSN="postgres://postgres@localhost:5432/postgres?sslmode=disable" \
    CLICKHOUSE_DSN="http://localhost:8123/default" \
    pgstats-to-clickhouse
```

- `INTERVAL` - collect interval in seconds (default: "30s", valid units are "ns", "us" (or "µs"), "ms", "s", "m", "h")
- `DISCOVERY_INTERVAL` - database discovery interval in seconds (default: "30s", valid units are "ns", "us" (or "µs"), "ms", "s", "m", "h")
- `POSTGRES_DSN` - connection to PostgreSQL (default: "postgres://postgres@localhost:5432/postgres?sslmode=disable")
- `CLICKHOUSE_DSN` - connection to Clickhouse (default: "http://localhost:8123/default"). As `schema://user:password@host[:port]/database?param1=value1&...&paramN=valueN`
    - example: `http://user:password@host:8123/database?timeout=5s&read_timeout=10s&write_timeout=20s`


## Demo:
```bash
make up
make build

env INTERVAL="30s" \
    DISCOVERY_INTERVAL="30s" \
    POSTGRES_DSN="postgres://postgres@localhost:5432/postgres?sslmode=disable" \
    CLICKHOUSE_DSN="http://localhost:8123/default" \
    ./bin/pgstats-to-clickhouse

docker-compose -p pgstats -f ./docker/docker-compose.yaml exec -T postgres su postgres -c 'createdb test'

docker-compose -p pgstats -f ./docker/docker-compose.yaml exec -T postgres su postgres -c 'pgbench -i test'

docker-compose -p pgstats -f ./docker/docker-compose.yaml exec -T postgres su postgres -c 'pgbench test'
...
# wait 30 seconds
...
docker-compose -p pgstats -f ./docker/docker-compose.yaml exec -T postgres su postgres -c 'pgbench test'
```
Open Grafana - http://localhost:3000/d/eec56kt8auhvkd/pg-stat-statements

## Requirements

Tested on:
```
Clickhouse 23.3
PostgreSQL 14+
Go 1.14
```

- postgres user should have atleast `pg_monitor` role, otherwise it will fail with error about queryid is NULL
- postgres user should have connect grants

## Known possible issues
- hash collision is not handled
- possible data loss or not honest metrics after `pg_stat_statements_reset()`
- hangs on during network calls if packets are dropped. Can't be interrapted by SIGINT (solved by connect_timeout)
- data loss if clickhouse is not accessable
- can open up to 3 connection in a once, if 3 collectors are used

## Caveats
- pg_stat_stamenents file on disk can be too huge and it causes disk swap during reading pg_stat_statements' view. Use small value `pg_stat_statements.max`
- in case of 1000+ amount of schemas and tables `pgstats-to-clickhouse` daemon may use too much memory, because it holds previous metrics snapshot (1 table = 1 entry)
