pipeline {
  agent any

  environment {
    // Use a DNS name in production if Harbor has one; IP shown matches the current infrastructure map.
    HARBOR_REGISTRY = '192.168.100.58'
    HARBOR_PROJECT = 'tropical'
    IMAGE_TAG = "${env.GIT_COMMIT ? env.GIT_COMMIT.take(12) : 'dev'}"
  }

  stages {
    stage('Checkout') { steps { checkout scm } }

    stage('Backend Quality Gate') {
      steps {
        sh 'docker run --rm -v "$PWD:/src" -w /src golang:1.23-alpine sh -c "go mod download && go test ./... && go vet ./..."'
      }
    }

    stage('Frontend Build') {
      steps {
        sh 'docker build -f Dockerfile.web -t tropical-web-ci .'
      }
    }

    stage('Build Images') {
      when { branch 'main' }
      steps {
        script {
          def services = ['auth-service','audit-service','inventory-service','sales-service','dashboard-service','api-gateway']
          services.each { svc ->
            sh "docker build -f Dockerfile.backend --build-arg SERVICE=${svc} -t ${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${svc}:${IMAGE_TAG} ."
          }
          sh "docker tag tropical-web-ci ${HARBOR_REGISTRY}/${HARBOR_PROJECT}/web:${IMAGE_TAG}"
        }
      }
    }

    stage('Push Harbor') {
      when { branch 'main' }
      steps {
        withCredentials([usernamePassword(credentialsId: 'harbor-credentials', usernameVariable: 'HARBOR_USER', passwordVariable: 'HARBOR_PASS')]) {
          sh 'echo "$HARBOR_PASS" | docker login "$HARBOR_REGISTRY" -u "$HARBOR_USER" --password-stdin'
          script {
            def services = ['auth-service','audit-service','inventory-service','sales-service','dashboard-service','api-gateway','web']
            services.each { svc -> sh "docker push ${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${svc}:${IMAGE_TAG}" }
          }
        }
      }
    }
  }

  post { always { sh 'docker logout "$HARBOR_REGISTRY" || true' } }
}
