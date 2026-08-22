pipeline {
  agent any

  environment {
    HARBOR_REGISTRY = 'harbor-dt.co.id'
    HARBOR_PROJECT  = 'devops-apps'
  }

  stages {
    stage('Checkout') {
      steps {
        checkout scm
        script {
          env.IMAGE_TAG = sh(
            script: 'git rev-parse --short=12 HEAD',
            returnStdout: true
          ).trim()

          echo "IMAGE_TAG=${env.IMAGE_TAG}"
        }
      }
    }

    stage('Backend Quality Gate') {
      steps {
        sh '''
          docker run --rm \
            -v "$PWD:/src" \
            -w /src \
            golang:1.23-alpine \
            sh -c "go mod download && go test ./... && go vet ./..."
        '''
      }
    }

    stage('Frontend Build') {
      steps {
        sh 'docker build -f Dockerfile.web -t tropical-web-ci .'
      }
    }

    stage('Build Images') {
      when {
        branch 'main'
      }

      steps {
        script {
          def images = [
            'auth-service'      : 'tropical-auth',
            'audit-service'     : 'tropical-audit',
            'inventory-service' : 'tropical-inventory',
            'sales-service'     : 'tropical-sales',
            'chat-service'      : 'tropical-chat',
            'dashboard-service' : 'tropical-dashboard',
            'api-gateway'       : 'tropical-api-gateway'
          ]

          images.each { serviceName, imageName ->
            sh """
              docker build \
                -f Dockerfile.backend \
                --build-arg SERVICE=${serviceName} \
                -t ${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${imageName}:${IMAGE_TAG} \
                .
            """
          }

          sh """
            docker tag tropical-web-ci \
              ${HARBOR_REGISTRY}/${HARBOR_PROJECT}/tropical-web:${IMAGE_TAG}
          """
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
            credentialsId: 'harbor-credentials',
            usernameVariable: 'HARBOR_USER',
            passwordVariable: 'HARBOR_PASS'
          )
        ]) {
          sh '''
            echo "$HARBOR_PASS" | \
              docker login "$HARBOR_REGISTRY" \
              -u "$HARBOR_USER" \
              --password-stdin
          '''

          script {
            def images = [
              'tropical-auth',
              'tropical-audit',
              'tropical-inventory',
              'tropical-sales',
              'tropical-chat',
              'tropical-dashboard',
              'tropical-api-gateway',
              'tropical-web'
            ]

            images.each { imageName ->
              sh """
                docker push \
                  ${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${imageName}:${IMAGE_TAG}
              """
            }
          }
        }
      }
    }
  }

  post {
    always {
      sh 'docker logout "$HARBOR_REGISTRY" || true'
    }
  }
}
