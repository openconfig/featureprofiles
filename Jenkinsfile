pipeline {
    agent any

    tools {
        go 'go1.21.3'
    }
    
    environment {
        CGO_ENABLED = 1
    }

    stages {
        stage('Vet') {
            steps {
                sh  'go vet -copylocks=false ./...'
            }
        }
        
        stage('Build') {
            steps {
                sh 'go build ./...'
            }
        }
        
        stage('Lint') {
            steps {
                script {
                    def ret = sh(script: 'gofmt -d -s . ', returnStdout: true).trim()
                    if (ret) {
                        println ret
                        sh 'exit 1'
                    }
                }
            }
        }
        
        stage('Static Check') {
            steps {
                sh 'go install honnef.co/go/tools/cmd/staticcheck@2023.1.7'
                script {
                    def ret = sh(
                        script: '${GOPATH}/bin/staticcheck ./...', 
                        returnStdout: true
                    ).trim()
                    if (ret) {
                        println ret
                        sh 'exit 1'
                    }
                }
            }
        }
    }
}
