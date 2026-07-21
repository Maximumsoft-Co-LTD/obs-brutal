package util

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

func WriteJSONToWriter(w io.Writer, entry *domain.LogEntry) error {
	var buf bytes.Buffer
	buf.WriteByte('{')
	writeKV := func(key string, val interface{}, isFirst *bool) error {
		if !*isFirst {
			buf.WriteByte(',')
		}
		*isFirst = false
		buf.WriteString(strconv.Quote(key))
		buf.WriteByte(':')
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	}
	isFirst := true
	if err := writeKV("datetime", entry.Timestamp.Format(time.RFC3339Nano), &isFirst); err != nil {
		return err
	}
	if err := writeKV("level", entry.Level.String(), &isFirst); err != nil {
		return err
	}
	if err := writeKV("msg", entry.Msg, &isFirst); err != nil {
		return err
	}
	if entry.TraceID != "" {
		if err := writeKV("trace_id", entry.TraceID, &isFirst); err != nil {
			return err
		}
	}
	if entry.SpanID != "" {
		if err := writeKV("span_id", entry.SpanID, &isFirst); err != nil {
			return err
		}
	}
	if entry.RequestID != "" {
		if err := writeKV("request_id", entry.RequestID, &isFirst); err != nil {
			return err
		}
	}
	if entry.UserID != "" {
		if err := writeKV("user_id", entry.UserID, &isFirst); err != nil {
			return err
		}
	}
	if entry.Mod != "" {
		if err := writeKV("module", entry.Mod, &isFirst); err != nil {
			return err
		}
	}
	if entry.TenantID != "" {
		if err := writeKV("tenant_id", entry.TenantID, &isFirst); err != nil {
			return err
		}
	}
	reserved := map[string]struct{}{"datetime": {}, "level": {}, "msg": {}, "trace_id": {}, "span_id": {}, "request_id": {}, "user_id": {}, "module": {}, "tenant_id": {}, "context": {}}
	keys := make([]string, 0, len(entry.F))
	for k := range entry.F {
		if _, ok := reserved[k]; ok {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := writeKV(k, entry.F[k], &isFirst); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

func EncodeEntryToJSON(entry *domain.LogEntry) ([]byte, error) {
	var buf bytes.Buffer
	if err := WriteJSONToWriter(&buf, entry); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
