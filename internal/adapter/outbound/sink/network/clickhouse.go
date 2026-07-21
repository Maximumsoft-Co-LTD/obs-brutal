package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
)

type ClickHouseSink struct {
	port.SinkBase
	endpoint   string
	database   string
	table      string
	username   string
	password   string
	autoCreate bool
	client     *http.Client
}

func NewClickHouseSink() port.Sink {
	return &ClickHouseSink{endpoint: "http://localhost:8123", database: "default", table: "logs", autoCreate: true, client: &http.Client{}}
}

func (s *ClickHouseSink) Name() string { return "clickhouse" }

func (s *ClickHouseSink) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["endpoint"].(string); ok && v != "" {
		s.endpoint = v
	}
	if v, ok := cfg["database"].(string); ok && v != "" {
		s.database = v
	}
	if v, ok := cfg["table"].(string); ok && v != "" {
		s.table = v
	}
	if v, ok := cfg["username"].(string); ok {
		s.username = v
	}
	if v, ok := cfg["password"].(string); ok {
		s.password = v
	}
	if v, ok := cfg["auto_create"].(bool); ok {
		s.autoCreate = v
	}
	return nil
}

func (s *ClickHouseSink) ensureTable() {
	if !s.autoCreate {
		return
	}
	create := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
        datetime DateTime,
        level LowCardinality(String),
        msg String,
        trace_id String, span_id String, request_id String, user_id String,
        module String, tenant_id String, error String,
        fields String
    ) ENGINE = MergeTree ORDER BY (datetime, level)`, s.database, s.table)
	_ = s.execQuery(create)
}

func (s *ClickHouseSink) execQuery(query string) error {
	u, _ := url.Parse(s.endpoint)
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest("POST", u.String(), nil)
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("clickhouse status: %s", resp.Status)
	}
	return nil
}

func (s *ClickHouseSink) Write(entry *domain.LogEntry) error {
	if entry == nil {
		return nil
	}
	s.ensureTable()
	fieldsJSON, _ := json.Marshal(entry.F)
	rowMap := map[string]interface{}{
		"datetime":   entry.Timestamp.Format("2006-01-02 15:04:05"),
		"level":      entry.Level.String(),
		"msg":        entry.Msg,
		"trace_id":   firstStr(entry.TraceID, entry.F["trace_id"]),
		"span_id":    firstStr(entry.SpanID, entry.F["span_id"]),
		"request_id": firstStr(entry.RequestID, entry.F["request_id"]),
		"user_id":    firstStr(entry.UserID, entry.F["user_id"]),
		"module":     firstStr(entry.Mod, entry.F["module"]),
		"tenant_id":  firstStr(entry.TenantID, entry.F["tenant_id"]),
		"error":      errorStr(entry),
		"fields":     string(fieldsJSON),
	}
	row, err := json.Marshal(rowMap)
	if err != nil {
		return err
	}
	u, _ := url.Parse(s.endpoint)
	q := u.Query()
	q.Set("query", fmt.Sprintf("INSERT INTO %s.%s FORMAT JSONEachRow", s.database, s.table))
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest("POST", u.String(), bytes.NewReader(row))
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("clickhouse insert: %s", resp.Status)
	}
	return nil
}

func firstStr(primary string, alt interface{}) string {
	if primary != "" {
		return primary
	}
	if v, ok := alt.(string); ok {
		return v
	}
	return ""
}

func errorStr(entry *domain.LogEntry) string {
	if entry.Err != nil {
		return entry.Err.Error()
	}
	if v, ok := entry.F["error"].(string); ok {
		return v
	}
	return ""
}
