#!/bin/bash

REGISTRY="harbor-dt.co.id/devops-apps"

images=(
"tropical-api-gateway"
"tropical-auth"
"tropical-audit"
"tropical-inventory"
"tropical-sales"
"tropical-dashboard"
"tropical-chat"
"tropical-web"
)

for image in "${images[@]}"
do

 echo "Pushing ${image}"

 docker push ${REGISTRY}/${image}:v1

done

echo "ALL PUSH FINISHED"
