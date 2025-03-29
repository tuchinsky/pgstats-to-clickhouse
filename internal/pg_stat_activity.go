package internal

import (
	"database/sql"
	"fmt"
)

type PgStatActivityFactory struct{}

type PgStatActivity struct {
	datname               string
	pid                   int64
	username              sql.NullString
	application_name      sql.NullString
	xact_start            string
	query_start           string
	state                 string
	query                 string
	query_id              sql.NullString
	backend_type          sql.NullString
	wait_event_type       sql.NullString
	wait_event            sql.NullString
	xact_duration         float64
	query_duration        float64
	state_change_duration float64
}

func (f *PgStatActivityFactory) Name() string {
	return "PgStatActivity"
}

func (f *PgStatActivityFactory) CollectQuery() string {
	//main query to get metrics
	return `SELECT
				datname,
				pid,
				usename as username,
				application_name,
				xact_start,
				query_start,
				state,
				query,
				query_id,
				backend_type,
				wait_event_type,
				wait_event,
				extract(epoch from now() - xact_start)::bigint as xact_duration,
				extract(epoch from now() - query_start)::bigint as query_duration,
				extract(epoch from now() - state_change)::bigint as state_change_duration
			FROM pg_stat_activity
			WHERE pid <> pg_backend_pid()
				AND state <> 'idle'
				AND query IS NOT null
				AND backend_type NOT IN ('walsender', 'checkpointer', 'walwriter')
				AND extract(epoch from now() - xact_start) > 30
			ORDER BY datname, username, query`
}

func (f *PgStatActivityFactory) PushQuery() string {
	//query to store in clickhouse populated data with hostname
	return `INSERT INTO pgmetrics.pg_stat_activity_buffer(
					hostname,
					datname,
					pid,
					username,
					application_name,
					xact_start,
					query_start,
					state,
					query,
					query_id,
					backend_type,
					wait_event_type,
					wait_event,
					xact_duration,
					query_duration,
					state_change_duration) VALUES (
						?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
					)`
}

func (f *PgStatActivityFactory) NewMetric(rows *sql.Rows) (PgMetric, error) {
	metric := new(PgStatActivity)
	err := rows.Scan(
		&metric.datname,
		&metric.pid,
		&metric.username,
		&metric.application_name,
		&metric.xact_start,
		&metric.query_start,
		&metric.state,
		&metric.query,
		&metric.query_id,
		&metric.backend_type,
		&metric.wait_event_type,
		&metric.wait_event,
		&metric.xact_duration,
		&metric.query_duration,
		&metric.state_change_duration,
	)
	if err != nil {
		return nil, err
	}
	return metric, nil
}

func (p *PgStatActivity) isSkippable(old PgMetric) bool {
	_, ok := old.(*PgStatActivity)
	if !ok {
		panic(fmt.Sprintf("isSkippable: this is not PgStatActivity: %v", old))
	}
	return false
}

func (p *PgStatActivity) delta(old PgMetric) PgMetric {
	_, ok := old.(*PgStatActivity)
	if !ok {
		panic(fmt.Sprintf("delta: this is not PgStatActivity: %v", old))
	}

	return &PgStatActivity{
		datname:               p.datname,
		pid:                   p.pid,
		username:              p.username,
		application_name:      p.application_name,
		xact_start:            p.xact_start,
		query_start:           p.query_start,
		state:                 p.state,
		query:                 p.query,
		query_id:              p.query_id,
		backend_type:          p.backend_type,
		wait_event_type:       p.wait_event_type,
		wait_event:            p.wait_event,
		xact_duration:         p.xact_duration,
		query_duration:        p.query_duration,
		state_change_duration: p.state_change_duration,
	}
}

func (p *PgStatActivity) getHash() uint32 {
	return getHash(p.datname, p.query)
}

func (p *PgStatActivity) getValue(hostname string) []interface{} {
	return []interface{}{
		hostname,
		p.datname,
		p.pid,
		p.username,
		p.application_name,
		p.xact_start,
		p.query_start,
		p.state,
		p.query,
		p.query_id,
		p.backend_type,
		p.wait_event_type,
		p.wait_event,
		p.xact_duration,
		p.query_duration,
		p.state_change_duration,
	}
}
