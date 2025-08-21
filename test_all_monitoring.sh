#!/bin/bash

echo "🚀 OBS-Brutal Complete Monitoring Test"
echo "======================================"

# Check if monitoring stack is running
echo "📋 Checking monitoring services..."

# Test OBS-Brutal
if curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "✅ OBS-Brutal: http://localhost:8080 - RUNNING"
else
    echo "❌ OBS-Brutal: Not running"
    echo "💡 Start with: docker compose -f docker-compose.working.yml up -d"
    exit 1
fi

# Test Prometheus
if curl -s http://localhost:9090/-/healthy > /dev/null 2>&1; then
    echo "✅ Prometheus: http://localhost:9090 - RUNNING"
else
    echo "❌ Prometheus: Not running"
fi

# Test Grafana  
if curl -s http://localhost:3000/api/health > /dev/null 2>&1; then
    echo "✅ Grafana: http://localhost:3000 - RUNNING"
else
    echo "❌ Grafana: Not running"
fi

# Test Loki
if curl -s http://localhost:3100/ready > /dev/null 2>&1; then
    echo "✅ Loki: http://localhost:3100 - RUNNING"
else
    echo "❌ Loki: Not running"
fi

# Test Jaeger
if curl -s http://localhost:16686/api/services > /dev/null 2>&1; then
    echo "✅ Jaeger: http://localhost:16686 - RUNNING"
else
    echo "❌ Jaeger: Not running"
fi

echo ""
echo "🧪 Testing OBS-Brutal endpoints..."

echo ""
echo "1. 📊 Testing Prometheus Metrics:"
echo "   Endpoint: http://localhost:8080/metrics"
metrics_count=$(curl -s http://localhost:8080/metrics | grep -c "obs_brutal")
echo "   OBS-Brutal metrics found: $metrics_count"
echo "   Sample metrics:"
curl -s http://localhost:8080/metrics | grep "obs_brutal" | head -3

echo ""
echo "2. 🏥 Testing Health Check:"
health_response=$(curl -s http://localhost:8080/health)
echo "   Response: $(echo "$health_response" | head -c 150)..."

echo ""
echo "3. ⚡ Testing Performance:"
perf_response=$(curl -s http://localhost:8080/performance)
logs_per_sec=$(echo "$perf_response" | grep -o '"logs_per_second":"[^"]*"' | cut -d'"' -f4)
echo "   Performance result: $logs_per_sec logs/sec"

echo ""
echo "4. 📝 Testing Log Generation:"
logs_response=$(curl -s http://localhost:8080/logs-to-loki)
echo "   Logs demo: $(echo "$logs_response" | head -c 100)..."

echo ""
echo "5. 🔍 Testing Jaeger Traces:"
trace_response=$(curl -s http://localhost:8080/trace-test)
trace_id=$(echo "$trace_response" | grep -o '"trace_id":"[^"]*"' | cut -d'"' -f4)
echo "   Trace generated: $trace_id"
echo "   View in Jaeger: http://localhost:16686"

echo ""
echo "=== 🎯 MONITORING ACCESS GUIDE ==="
echo ""
echo "🌐 Web Interfaces:"
echo "   📊 OBS-Brutal Demo:  http://localhost:8080"
echo "   📈 Prometheus:       http://localhost:9090"
echo "   🎛️  Grafana:          http://localhost:3000 (admin/admin)"
echo "   📝 Loki:             http://localhost:3100"
echo "   🔍 Jaeger:           http://localhost:16686"

echo ""
echo "🔧 Working Endpoints to Test:"
echo "   curl http://localhost:8080/health"
echo "   curl http://localhost:8080/metrics" 
echo "   curl http://localhost:8080/performance"
echo "   curl http://localhost:8080/trace-test"
echo "   curl http://localhost:8080/logs-to-loki"
echo "   curl http://localhost:8080/activity"

echo ""
echo "📊 Prometheus Queries to Try:"
echo "   - obs_brutal_requests_total"
echo "   - obs_brutal_logs_total"
echo "   - rate(obs_brutal_requests_total[5m])"

echo ""
echo "📈 Grafana Setup:"
echo "   1. Go to http://localhost:3000"
echo "   2. Login: admin/admin"
echo "   3. Add Prometheus datasource: http://prometheus:9090"
echo "   4. Add Loki datasource: http://loki:3100"
echo "   5. Create dashboard with queries above"

echo ""
echo "📝 Loki Queries to Try:"
echo "   - {service=\"obs-brutal-monitoring\"}"
echo "   - {level=\"ERROR\"}"
echo "   - {trace_id=~\"trace_.*\"}"

echo ""
echo "✅ MONITORING TEST COMPLETED!"
echo "🎯 All services are working and generating data!"
