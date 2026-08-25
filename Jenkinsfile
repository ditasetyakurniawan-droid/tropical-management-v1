pipeline {
    agent any

    environment {
        HARBOR_REGISTRY = 'harbor-dt.co.id'
        HARBOR_PROJECT  = 'devops-apps'
        SONAR_HOST_URL  = 'http://sonar-dt:9000'
        SONAR_PROJECT   = 'tropical-management-v1'
        JENKINS_CONTAINER = 'jenkins-server'
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

                    // Validasi format semver: 1.0.0 / 1.0.0-beta1 / 1.0.0-rc1
                    if (!(rawVersion ==~ /^\d+\.\d+\.\d+(-[a-zA-Z0-9]+)?$/)) {
                        error "Format VERSION tidak valid: '${rawVersion}'. Contoh valid: 1.0.0 atau 1.0.0-beta1"
                    }

                    env.APP_VERSION = rawVersion
                    // Tag immutable & traceable ke commit persis
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
                      golang:1.23-bookworm \
                      bash -c '
                        go mod download &&
                        go test -coverprofile=coverage.out ./... &&
                        sed -i "s|github.com/ditasetyakurniawan-droid/tropical-management-v1/||g" coverage.out &&
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
                          -Dsonar.projectKey="$SONAR_PROJECT" \
                          -Dsonar.projectName="Tropical Management" \
                          -Dsonar.sources=. \
                          -Dsonar.exclusions="**/node_modules/**,**/.next/**,**/.git/**,**/coverage/**,**/*_test.go" \
                          -Dsonar.tests=. \
                          -Dsonar.test.inclusions="**/*_test.go" \
                          -Dsonar.go.coverage.reportPaths=coverage.out \
                          -Dsonar.qualitygate.wait=false
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
                        'api-gateway'       : 'tropical-api-gateway'
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
                          tropical-web
                        do
                            # Tag immutable (versi + sha) - source of truth utama, dipakai untuk traceability & rollback presisi
                            docker push \
                              "$HARBOR_REGISTRY/$HARBOR_PROJECT/$IMAGE:$IMAGE_TAG"

                            # Tag pointer human-readable (versi saja) - dipakai untuk referensi di manifest/ArgoCD
                            docker tag \
                              "$HARBOR_REGISTRY/$HARBOR_PROJECT/$IMAGE:$IMAGE_TAG" \
                              "$HARBOR_REGISTRY/$HARBOR_PROJECT/$IMAGE:$APP_VERSION"

                            docker push \
                              "$HARBOR_REGISTRY/$HARBOR_PROJECT/$IMAGE:$APP_VERSION"
                        done

                        docker logout "$HARBOR_REGISTRY" || true
                        rm -rf "$DOCKER_CONFIG"
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
            sh '''
                rm -rf "$WORKSPACE/.docker-ci" || true
                docker image prune -f || true
            '''
        }
    }
}
