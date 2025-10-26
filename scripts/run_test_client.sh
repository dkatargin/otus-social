#!/bin/bash

set -e

echo "Building test client..."
cd src/cmd/test_client
go build -o test_client main.go

echo ""
echo "Starting test client..."
echo "This will generate load on the dialog service:"
echo "  - 10 concurrent workers"
echo "  - 100 requests per second"
echo "  - Running for 60 seconds"
echo "  - User IDs: 1-100"
echo ""

./test_client \
  --url http://localhost:8080 \
  --workers 10 \
  --duration 60 \
  --rps 100 \
  --user-from 1 \
  --user-to 100

echo ""
echo "Test completed!"
echo ""
echo "Check metrics:"
echo "  Prometheus: http://localhost:9090"
echo "  Grafana: http://localhost:3000 (admin/admin)"
echo "  Zabbix: http://localhost:8081"
