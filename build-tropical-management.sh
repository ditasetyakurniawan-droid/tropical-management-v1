#!/bin/bash

REGISTRY="harbor-dt.co.id/devops-apps"

services=(
"api-gateway:tropical-api-gateway"
"auth-service:tropical-auth"
"audit-service:tropical-audit"
"inventory-service:tropical-inventory"
"sales-service:tropical-sales"
"dashboard-service:tropical-dashboard"
"chat-service:tropical-chat"
)

for service in "${services[@]}"
do

  SERVICE_NAME=${service%%:*}
  IMAGE_NAME=${service##*:}

  echo "====================================="
  echo "Building ${IMAGE_NAME}"
  echo "====================================="

  docker build \
    -f Dockerfile.backend \
    --build-arg SERVICE=${SERVICE_NAME} \
    -t ${REGISTRY}/${IMAGE_NAME}:v1 .

done


echo "====================================="
echo "Building web"
echo "====================================="

docker build \
-f Dockerfile.web \
-t ${REGISTRY}/tropical-web:v1 .


echo "ALL BUILD FINISHED"
