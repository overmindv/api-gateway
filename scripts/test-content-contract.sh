#!/usr/bin/env bash
set -euo pipefail

dsn=${CONTENT_CONTRACT_DSN:-}
if [[ ! $dsn =~ ^postgres://postgres@127\.0\.0\.1:[0-9]{1,5}/content_contract_test\?sslmode=disable$ ]]; then
  echo "CONTENT_CONTRACT_DSN must point to the isolated local content_contract_test database" >&2
  exit 2
fi

module=github.com/overmindv/content
source_dir=${CONTENT_CONTRACT_SOURCE:-}
if [[ -z $source_dir ]]; then
  go mod download "$module"
  source_dir=$(go list -m -f '{{.Dir}}' "$module")
fi
source_dir=$(cd "$source_dir" && pwd -P)

scratch=$(mktemp -d)
content_pid=
cleanup() {
  if [[ -n $content_pid ]]; then
    kill "$content_pid" 2>/dev/null || true
    wait "$content_pid" 2>/dev/null || true
  fi
  rm -f -- "$scratch/content"
  rmdir -- "$scratch"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

(
  cd "$source_dir"
  test "$(go list -m)" = "$module"
  go build -mod=readonly -o "$scratch/content" ./cmd/content
)
export CONTENT_CONTRACT_GRPC_ADDR=127.0.0.1:19196
CONTENT_CONTRACT_TOKEN=$(openssl rand -hex 32)
export CONTENT_CONTRACT_TOKEN

(
  cd "$scratch"
  env -i PATH="$PATH" "$scratch/content" migrate --dir "$source_dir/migrations/schema" --dsn "$dsn" up
  exec env -i PATH="$PATH" ENV=local LOG_LEVEL=warn DATABASE_URL="$dsn" KAFKA_ENABLED=false \
    HTTP_ADDR=127.0.0.1:19197 GRPC_ADDR="$CONTENT_CONTRACT_GRPC_ADDR" \
    CONTENT_GRPC_SERVICE_TOKEN="$CONTENT_CONTRACT_TOKEN" "$scratch/content"
) &
content_pid=$!

ready=false
for ((i=0; i<60; i++)); do
  kill -0 "$content_pid" 2>/dev/null || break
  if curl --fail --silent --max-time 1 http://127.0.0.1:19197/ready >/dev/null &&
    (: >/dev/tcp/127.0.0.1/19196) 2>/dev/null; then
    ready=true
    break
  fi
  sleep 0.25
done
if [[ $ready != true ]]; then
  echo "Content did not start on the loopback fixture ports" >&2
  exit 1
fi

go test -race -count=1 -timeout=45s -run '^TestMaterialContentContract$' -v ./tests/integration
