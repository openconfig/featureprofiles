pipeline {
    agent any

    tools {
        go 'go1.24.7'
    }
    
    environment {
        CGO_ENABLED = 1
    }

    stages {
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
                script {
                    // the target branch might not always be master, it could also sometimes be main, so we have to check the PR data itself to get the target branch
                    sh "git fetch origin ${ghprbTargetBranch}"
                    def changedFiles = sh(script: "git diff --name-only \$(git merge-base origin/${ghprbTargetBranch} ${ghprbActualCommit}) ${ghprbActualCommit}", returnStdout: true).trim().split('\n')
                    echo "Changed files: ${changedFiles}"

                    // check each file in changedFiles, if none of them ends with .go, then return
                    def goFileChanged = false
                    for (file in changedFiles) {
                        if (file.endsWith('.go')) {
                            goFileChanged = true
                            break
                        }
                    }
                    if (!goFileChanged) {
                        echo "No Go files changed, skipping staticcheck."
                        return
                    }

                    def packages = sh(script: 'go list ./...', returnStdout: true).trim().split('\n')
                    def pkgSet = [] as Set

                    // Some files in changedFiles might be part of the same package, so we use a set to avoid duplicates
                    for (file in changedFiles) {
                        if (!file.endsWith('.go')) {
                            continue
                        }
                        for (pkg in packages) {
                            def pkgPath = pkg.replace('github.com/openconfig/featureprofiles/', '')
                            if (file.startsWith(pkgPath + '/') || file == pkgPath) {
                                pkgSet.add(pkg)
                                break
                            }
                        }
                    }

                    echo "Changed Go packages: ${pkgSet}"

                    if (pkgSet.isEmpty()) {
                        // would only happen if there are changed go files not part of any package,
                        // staticcheck wouldn't be able to properly parse them
                        echo "No Go packages changed, skipping staticcheck."
                    } else {
                        sh 'go install honnef.co/go/tools/cmd/staticcheck@2025.1.1'
                        def pkgCmd = pkgSet.join(' ')
                        echo "Running staticcheck on packages: ${pkgCmd}"
                        sh "staticcheck ${pkgCmd}"
                    }
                }
            }
        }
    }
}
