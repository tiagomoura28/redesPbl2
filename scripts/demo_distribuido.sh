#!/usr/bin/env sh
set -eu
# Execute depois de: docker compose up --build -d empresa1 empresa2 empresa3
# IDs unicos permitem repetir a demonstracao sem apagar os volumes.
suffix="$(date +%s)"
c1="C1-${suffix}"; c2="C2-${suffix}"; c3="C3-${suffix}"
for port in 8081 8082 8083; do
  i=0
  until curl -fsS "http://localhost:${port}/api/v1/health" >/dev/null; do
    i=$((i+1)); [ "$i" -lt 30 ] || { echo "Servidor na porta ${port} indisponivel"; exit 1; }
    sleep 1
  done
done
curl -fsS -X POST http://localhost:8081/api/v1/rides -H 'Content-Type: application/json' -d "{\"id\":\"${c1}\",\"driver_id\":\"motorista-a\",\"route\":[\"Salvador\",\"Feira de Santana\"],\"date\":\"2026-11-10\",\"seats\":2,\"price_per_segment\":30}"
echo
curl -fsS -X POST http://localhost:8082/api/v1/rides -H 'Content-Type: application/json' -d "{\"id\":\"${c2}\",\"driver_id\":\"motorista-b\",\"route\":[\"Feira de Santana\",\"Vitoria da Conquista\"],\"date\":\"2026-11-10\",\"seats\":2,\"price_per_segment\":45}"
echo
curl -fsS -X POST http://localhost:8083/api/v1/rides -H 'Content-Type: application/json' -d "{\"id\":\"${c3}\",\"driver_id\":\"motorista-c\",\"route\":[\"Vitoria da Conquista\",\"Montes Claros\"],\"date\":\"2026-11-10\",\"seats\":2,\"price_per_segment\":60}"
echo
payload="{\"passenger_id\":\"passageiro-${suffix}\",\"legs\":[{\"server_url\":\"http://empresa1:8081\",\"ride_id\":\"${c1}\",\"origin\":\"Salvador\",\"destination\":\"Feira de Santana\"},{\"server_url\":\"http://empresa2:8081\",\"ride_id\":\"${c2}\",\"origin\":\"Feira de Santana\",\"destination\":\"Vitoria da Conquista\"},{\"server_url\":\"http://empresa3:8081\",\"ride_id\":\"${c3}\",\"origin\":\"Vitoria da Conquista\",\"destination\":\"Montes Claros\"}]}"
curl -i -X POST http://localhost:8081/api/v1/reservations -H 'Content-Type: application/json' -d "$payload"
