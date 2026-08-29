pipeline {
    agent any

    environment {
        HARBOR_REGISTRY   = 'harbor-dt.co.id'
        HARBOR_PROJECT    = 'devops-apps'
        SONAR_HOST_URL    = 'http://sonar-dt:9000'
        SONAR_PROJECT     = 'tropical-management-v1'
        JENKINS_CONTAINER = 'jenkins-server'
        MIN_GO_COVERAGE   = '65.0'

        // GitOps
        GITOPS_REPO       = 'github.com/ditasetyakurniawan-droid/tropical-management-gitops.git'
        GITOPS_OVERLAY    = 'apps/tropical-management/overlays/test-app'
    }

    options {
        timestamps()
        disableConcurrentBuilds()
        skipDefaultCheckout(true)
    }

    stages {

        stage('Checkout') {
            steps {
                checkout scm

                script {
                    env.GIT_SHORT_SHA = sh(
                        script: 'git rev-parse --short=12 HEAD',
                        returnStdout: true
                    ).trim()

                    echo "GIT_SHORT_SHA=${GIT_SHORT_SHA}"
                }
            }
        }

        stage('Resolve Version') {
            steps {
                script {
                    if (!fileExists('VERSION')) {
                        error "File VERSION tidak ditemukan di root repo. Buat file VERSION berisi versi semver, contoh: 1.0.0-beta1"
                    }

                    def rawVersion = readFile('VERSION').trim()

                    if (!(rawVersion ==~ /^\d+\.\d+\.\d+(-[a-zA-Z0-9]+)?$/)) {
                        error "Format VERSION tidak valid: '${rawVersion}'. Contoh valid: 1.0.0 atau 1.0.0-beta1"
                    }

                    env.APP_VERSION = rawVersion
                    env.IMAGE_TAG   = "${rawVersion}-${env.GIT_SHORT_SHA}"

                    echo "APP_VERSION=${env.APP_VERSION}"
                    echo "IMAGE_TAG=${env.IMAGE_TAG}"
                }
            }
        }

        stage('Backend Test') {
            steps {
                sh '''
                    docker run --rm \
                      --volumes-from "$JENKINS_CONTAINER" \
                      -w "$WORKSPACE" \
                      -e HOME=/tmp \
                      -e MIN_GO_COVERAGE="$MIN_GO_COVERAGE" \
                      golang:1.23-bookworm \
                      bash -c '
                        set -euo pipefail
                        go mod download
                        go mod verify
                        go test -json -race -covermode=atomic -coverprofile=coverage.out ./... | tee go-test.json
                        test -s coverage.out
                        ./scripts/normalize-go-coverage.sh coverage.out coverage.sonar.out
                        test -s coverage.sonar.out
                        test -s go-test.json
                        go tool cover -func=coverage.out | tee coverage-summary.txt
                        TOTAL_COVERAGE=$(go tool cover -func=coverage.out | tail -n 1 | grep -oE "[0-9]+([.][0-9]+)?%" | tr -d "%")
                        test -n "$TOTAL_COVERAGE"
                        echo "Total Go coverage: ${TOTAL_COVERAGE}% (minimum ${MIN_GO_COVERAGE}%)"
                        awk -v coverage="$TOTAL_COVERAGE" -v minimum="$MIN_GO_COVERAGE" "BEGIN { if ((coverage + 0) < (minimum + 0)) exit 1 }"
                        go vet ./...
                      '
                '''
            }
        }

        stage('Frontend Build') {
            steps {
                sh '''
                    docker run --rm \
                      --volumes-from "$JENKINS_CONTAINER" \
                      -w "$WORKSPACE/web" \
                      -u "$(id -u):$(id -g)" \
                      -e HOME=/tmp \
                      node:22-bookworm-slim \
                      sh -c '
                        set -e
                        npm ci
                        npm run test:coverage
                        test -s coverage/lcov.info
                        npm run build
                        rm -rf node_modules .next
                      '
                '''
            }
        }

        stage('SonarQube') {
            when {
                branch 'main'
            }

            steps {
                withCredentials([
                    string(
                        credentialsId: 'sonar-tropical-token',
                        variable: 'SONAR_TOKEN'
                    )
                ]) {
                    sh '''
                        docker run --rm \
                          --add-host sonar-dt:192.168.100.59 \
                          --volumes-from "$JENKINS_CONTAINER" \
                          -w "$WORKSPACE" \
                          -e SONAR_TOKEN="$SONAR_TOKEN" \
                          sonarsource/sonar-scanner-cli:latest \
                          -Dsonar.host.url="$SONAR_HOST_URL" \
                          -Dsonar.qualitygate.wait=true \
                          -Dsonar.qualitygate.timeout=300
                    '''
                }
            }
        }

        stage('Build Images') {
            when {
                branch 'main'
            }

            steps {
                script {
                    def services = [
                        'auth-service'      : 'tropical-auth',
                        'audit-service'     : 'tropical-audit',
                        'inventory-service' : 'tropical-inventory',
                        'sales-service'     : 'tropical-sales',
                        'chat-service'      : 'tropical-chat',
                        'dashboard-service' : 'tropical-dashboard',
                        'api-gateway'       : 'tropical-api-gateway',
                        'db-migrator'       : 'tropical-db-migrator'
                    ]

                    services.each { service, image ->
                        sh """
                            docker build \
                              -f Dockerfile.backend \
                              --build-arg SERVICE=${service} \
                              -t ${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${image}:${IMAGE_TAG} \
                              .
                        """
                    }

                    sh '''
                        docker build \
                          -f Dockerfile.web \
                          --build-arg NEXT_PUBLIC_API_URL="" \
                          -t "$HARBOR_REGISTRY/$HARBOR_PROJECT/tropical-web:$IMAGE_TAG" \
                          .
                    '''
                }
            }
        }

        // ============================================================
        // CHANGED: Hapus push floating tag $APP_VERSION
        // Hanya push tag unik $IMAGE_TAG (versi+sha)
        // Aman dengan Harbor immutability ON
        // ============================================================
        stage('Push Harbor') {
            when {
                branch 'main'
            }

            steps {
                withCredentials([
                    usernamePassword(
                        credentialsId: 'harbor-cred',
                        usernameVariable: 'HARBOR_USER',
                        passwordVariable: 'HARBOR_PASS'
                    )
                ]) {
                    sh '''
                        set -eu

                        export DOCKER_CONFIG="$WORKSPACE/.docker-ci"
                        rm -rf "$DOCKER_CONFIG"
                        mkdir -p "$DOCKER_CONFIG"

                        echo "$HARBOR_PASS" |
                          docker login "$HARBOR_REGISTRY" \
                            -u "$HARBOR_USER" \
                            --password-stdin

                        for IMAGE in \
                          tropical-auth \
                          tropical-audit \
                          tropical-inventory \
                          tropical-sales \
                          tropical-chat \
                          tropical-dashboard \
                          tropical-api-gateway \
                          tropical-db-migrator \
                          tropical-web
                        do
                            docker push "$HARBOR_REGISTRY/$HARBOR_PROJECT/$IMAGE:$IMAGE_TAG"
                        done

                        docker logout "$HARBOR_REGISTRY" || true
                        rm -rf "$DOCKER_CONFIG"
                    '''
                }
            }
        }

        // ============================================================
        // NEW: Auto-update image tag di repo GitOps
        // Jenkins clone -> sed update newTag -> commit -> push
        // ArgoCD/Flux akan auto-sync setelah detect perubahan
        // ============================================================
        stage('Update GitOps') {
            when {
                branch 'main'
            }

            steps {
                withCredentials([
                    usernamePassword(
                        credentialsId: 'tropical-jenkins-gitops',
                        usernameVariable: 'GITHUB_USER',
                        passwordVariable: 'GITHUB_TOKEN'
                    )
                ]) {
                    sh '''
                        set -eu

                        rm -rf gitops-repo

                        git clone https://${GITHUB_USER}:${GITHUB_TOKEN}@${GITOPS_REPO} gitops-repo

                        cd gitops-repo/${GITOPS_OVERLAY}

                        # Update semua newTag sekaligus
                        sed -i "s|newTag: .*|newTag: ${IMAGE_TAG}|g" kustomization.yaml

                        echo "=== Updated kustomization.yaml ==="
                        cat kustomization.yaml
                        echo "==================================="

                        git config user.email "jenkins@dt.co.id"
                        git config user.name "Jenkins CI"

                        git add kustomization.yaml

                        # Commit hanya jika ada perubahan
                        if git diff --cached --quiet; then
                            echo "Tidak ada perubahan tag, skip commit."
                        else
                            git commit -m "chore(test-app): bump images to ${IMAGE_TAG}"
                            git push origin main
                            echo "GitOps repo updated -> ${IMAGE_TAG}"
                        fi
                    '''
                }
            }
        }
    }

    post {
        success {
            echo "TROPICAL CI SUCCESS - VERSION: ${APP_VERSION} | IMAGE TAG: ${IMAGE_TAG}"
        }

        failure {
            echo "TROPICAL CI FAILED"
        }

        always {
            archiveArtifacts artifacts: 'coverage.out,coverage.sonar.out,coverage-summary.txt,go-test.json,web/coverage/lcov.info', allowEmptyArchive: true
            sh '''
                rm -rf "$WORKSPACE/.docker-ci" || true
                rm -rf "$WORKSPACE/gitops-repo" || true
                docker image prune -f || true
            '''
        }
    }
}