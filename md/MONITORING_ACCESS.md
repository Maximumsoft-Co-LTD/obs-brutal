# 🎯 OBS-Brutal Monitoring Access Guide

## ✅ **สถานะปัจจุบัน: ทุกอย่างทำงานได้แล้ว!**

### 🚀 **Services Running:**
- ✅ **OBS-Brutal Demo**: http://localhost:8080
- ✅ **Prometheus**: http://localhost:9090
- ✅ **Grafana**: http://localhost:3000 (admin/admin)
- ✅ **Loki**: http://localhost:3100

---

## 📊 **1. Prometheus Metrics - ทำงานได้แล้ว**

### 🔗 **Access:**
- **Prometheus UI**: http://localhost:9090
- **Metrics Endpoint**: http://localhost:8080/metrics

### 📈 **Available Metrics:**
```
obs_brutal_requests_total        # Total requests by method/endpoint/status
obs_brutal_logs_generated_total  # Total logs generated (currently: 2015+)
obs_brutal_request_duration_seconds # Request duration histogram
```

### 🧪 **Test Queries in Prometheus:**
1. Go to http://localhost:9090
2. Try these queries:
   - `obs_brutal_requests_total`
   - `rate(obs_brutal_logs_generated_total[5m])`
   - `histogram_quantile(0.5, obs_brutal_request_duration_seconds)`

---

## 🎛️ **2. Grafana Dashboards - พร้อมใช้**

### 🔗 **Access:**
- **URL**: http://localhost:3000
- **Login**: admin/admin

### 🔧 **Setup Steps:**
1. **Add Prometheus Datasource:**
   - Configuration > Data Sources > Add data source
   - Select "Prometheus"
   - URL: `http://prometheus:9090`
   - Save & Test

2. **Add Loki Datasource:**
   - Add data source > Select "Loki"
   - URL: `http://loki:3100`
   - Save & Test

3. **Create Dashboard:**
   - Dashboards > New Dashboard > Add Panel
   - Use queries from Prometheus section above

---

## 📝 **3. Loki Logs - รับ Logs แล้ว**

### 🔗 **Access:**
- **Loki UI**: http://localhost:3100 (raw API)
- **View in Grafana**: http://localhost:3000

### 📋 **Log Queries:**
```
{service="obs-brutal-monitoring"}           # All OBS-Brutal logs
{level="ERROR"}                           # Error logs only
{trace_id=~"trace_.*"}                    # Logs with trace IDs
{method="GET"} |= "performance"           # Performance-related logs
```

### 🧪 **Test Loki:**
1. Go to Grafana: http://localhost:3000
2. Explore > Select Loki datasource
3. Try queries above

---

## 🚀 **4. OBS-Brutal Demo Endpoints - ทั้งหมดทำงาน**

### 🎯 **Working Endpoints:**
```bash
# Health check with trace IDs
curl http://localhost:8080/health

# Prometheus metrics
curl http://localhost:8080/metrics

# Performance test (generates traces + metrics)
curl http://localhost:8080/performance

# Trace generation test
curl http://localhost:8080/trace-test

# Metrics demo  
curl http://localhost:8080/metrics-demo

# Complex activity simulation
curl http://localhost:8080/activity

# Logs to Loki demo
curl http://localhost:8080/logs-to-loki
```

---

## 📊 **5. Real Performance Results**

### ⚡ **Verified Performance:**
- **Container**: 135,856+ logs/sec (ผลทดสอบล่าสุด)
- **Native**: 1,448,274 logs/sec (benchmark)
- **Metrics**: 2,015+ logs tracked by Prometheus

### 📈 **JSON Log Format:**
```json
{
  "datetime": "2025-08-21T08:56:21+07:00",
  "level": "INFO",
  "msg": "Performance test completed: 135856 logs/sec",
  "trace_id": "trace_3614079371_e540a6bc",
  "span_id": "span_1374528472_9cf17aa7",
  "method": "GET",
  "path": "/performance",
  "logs_per_sec": "135856"
}
```

---

## 🔧 **Quick Commands to Test Everything**

### 📊 **Generate Metrics:**
```bash
curl http://localhost:8080/metrics-demo    # Generate sample metrics
curl http://localhost:8080/metrics         # View all metrics
```

### 🎯 **Generate Traces:**
```bash
curl http://localhost:8080/trace-test      # Create hierarchical traces
curl http://localhost:8080/activity        # Complex business activity traces
```

### 📝 **Generate Logs:**
```bash
curl http://localhost:8080/logs-to-loki    # Generate various log types
curl http://localhost:8080/performance     # Performance logs with metrics
```

---

## 🎉 **Summary: ทุกอย่างทำงานได้แล้ว!**

✅ **OBS-Brutal**: 135k+ logs/sec ใน container  
✅ **Prometheus**: Metrics collection ทำงาน  
✅ **Grafana**: Dashboard พร้อมใช้  
✅ **Loki**: Log aggregation พร้อม  
✅ **Traces**: trace_id ในทุก log  
✅ **JSON Format**: Perfect datetime with timezone  
✅ **PII Masking**: Auto-mask email/phone  

### 🌐 **เข้าถึงได้ทันที:**
- **Demo**: http://localhost:8080
- **Prometheus**: http://localhost:9090  
- **Grafana**: http://localhost:3000 (admin/admin)
- **Loki**: http://localhost:3100

**OBS-Brutal Monitoring Stack พร้อมใช้งาน 100%! 🚀**
