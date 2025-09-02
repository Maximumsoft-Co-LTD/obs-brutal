package util

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"strconv"

	"obs-brutal/internal/core/domain"
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
	_ = writeKV("datetime", entry.Timestamp.Format("2006-01-02T15:04:05-07:00"), &isFirst)
	_ = writeKV("level", entry.Level.String(), &isFirst)
	_ = writeKV("msg", entry.Msg, &isFirst)
	if entry.TraceID != "" {
		_ = writeKV("trace_id", entry.TraceID, &isFirst)
	}
	if entry.SpanID != "" {
		_ = writeKV("span_id", entry.SpanID, &isFirst)
	}
	if entry.RequestID != "" {
		_ = writeKV("request_id", entry.RequestID, &isFirst)
	}
	reserved := map[string]struct{}{"datetime": {}, "level": {}, "msg": {}, "trace_id": {}, "span_id": {}, "request_id": {}, "context": {}}
	keys := make([]string, 0, len(entry.F))
	for k := range entry.F {
		if _, ok := reserved[k]; ok {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_ = writeKV(k, entry.F[k], &isFirst)
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
