#!/usr/bin/env bash
set -euo pipefail

failures=0

check_forbidden() {
  local pkg="$1"
  shift
  local pattern="$1"
  shift
  local reason="$1"

  local matches
  matches=$(go list -deps -test=false "$pkg" | rg "$pattern" || true)
  if [[ -n "$matches" ]]; then
    echo "[FAIL] $pkg: $reason"
    echo "$matches" | sed 's/^/  - /'
    failures=$((failures + 1))
    return
  fi
  echo "[ OK ] $pkg"
}

check_exists() {
  local pkg="$1"
  if ! go list "$pkg" >/dev/null 2>&1; then
    echo "[FAIL] $pkg: package does not build"
    failures=$((failures + 1))
    return
  fi
  echo "[ OK ] $pkg exists"
}

echo "== dependency boundary checks =="

check_forbidden "./errors" '^(google\.golang\.org/grpc|google\.golang\.org/protobuf)' "errors root must not depend on gRPC/protobuf; use errors/grpcx"
check_forbidden "./auth" '^(google\.golang\.org/grpc|google\.golang\.org/protobuf)' "auth root must not depend on gRPC/protobuf; use auth/grpcx"
check_forbidden "./auth/jwt" '^(google\.golang\.org/grpc|google\.golang\.org/protobuf)' "auth/jwt root must not depend on gRPC/protobuf; use auth/jwt/grpcx"
check_forbidden "./notify" '(^github\.com/Tsukikage7/servex/v2/messaging/jobqueue$|^github\.com/google/uuid$)' "notify root must not depend on jobqueue/uuid; use notify/jobqueuex"
check_forbidden "./storage/cache" '(^github\.com/redis/go-redis|^github\.com/go-redis/redis)' "storage/cache root must not depend on Redis SDK; use storage/cache/redis"
check_forbidden "./discovery" '(^github\.com/hashicorp/consul/api$|^go\.etcd\.io/etcd/client/v3$|^github\.com/nacos-group/nacos-sdk-go/v2/clients$|^github\.com/google/uuid$|^google\.golang\.org/grpc$|^google\.golang\.org/protobuf)' "discovery root must not depend on provider SDKs, uuid, or gRPC/protobuf"
check_forbidden "./messaging/pubsub/factory" '(^github\.com/redis/go-redis|^github\.com/go-redis/redis|^github\.com/confluentinc/confluent-kafka-go|^github\.com/rabbitmq/amqp091-go$)' "pubsub factory root must not depend on provider SDKs"
check_forbidden "./messaging/jobqueue/factory" '(^github\.com/redis/go-redis|^github\.com/go-redis/redis|^github\.com/IBM/sarama$|^github\.com/rabbitmq/amqp091-go$|^gorm\.io/gorm$)' "jobqueue factory root must not depend on provider SDKs"
check_forbidden "./xutil/pagination" '^gorm\.io/gorm$' "xutil/pagination root must not depend on GORM; use xutil/pagination/gorm"
check_forbidden "./xutil/sorting" '^gorm\.io/gorm$' "xutil/sorting root must not depend on GORM; use xutil/sorting/gorm"
check_forbidden "./testx" '(^google\.golang\.org/grpc|^google\.golang\.org/protobuf|^github\.com/testcontainers/testcontainers-go)' "testx root must not depend on gRPC/protobuf/testcontainers; use testx/grpcx or testx/container"
check_forbidden "./transport" '^(google\.golang\.org/grpc|google\.golang\.org/protobuf)' "transport root must stay protocol-neutral; use transport/grpcx or concrete transport packages"

echo
echo "== adapter package availability =="
check_exists "./errors/grpcx"
check_exists "./auth/grpcx"
check_exists "./auth/jwt/grpcx"
check_exists "./notify/jobqueuex"
check_exists "./storage/cache/redis"
check_exists "./discovery/consul"
check_exists "./discovery/etcd"
check_exists "./discovery/nacos"
check_exists "./messaging/pubsub/factory/redis"
check_exists "./messaging/pubsub/factory/kafka"
check_exists "./messaging/pubsub/factory/rabbitmq"
check_exists "./messaging/jobqueue/factory/redis"
check_exists "./messaging/jobqueue/factory/kafka"
check_exists "./messaging/jobqueue/factory/rabbitmq"
check_exists "./messaging/jobqueue/factory/database"
check_exists "./xutil/pagination/gorm"
check_exists "./xutil/sorting/gorm"
check_exists "./testx/grpcx"

if [[ "$failures" -ne 0 ]]; then
  echo
  echo "dependency boundary checks failed: $failures"
  exit 1
fi

echo
echo "dependency boundary checks passed"
