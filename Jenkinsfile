pipeline {
    agent any

    tools {
        go 'go1.21.3'
    }
    
    environment {
        GIT_LFS_SKIP_SMUDGE = 1
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
                sh 'go install honnef.co/go/tools/cmd/staticcheck@latest'
                script {
                    def ret = sh(
                        script: 'go list ./... | grep -F -v -e /python -e /fluent | xargs ${GOPATH}/bin/staticcheck', 
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
