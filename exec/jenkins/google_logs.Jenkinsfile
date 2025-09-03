pipeline {
    agent { label "ads" }

    parameters {
        string(
            defaultValue: '',
            description: 'List of FireX run ids separated by a comma',
            name: 'firex_ids', 
            trim: true
        )
        
        string(
            defaultValue: '/auto/tftpboot-ottawa/b4/jenkins/google_logs_dropbox', 
            description: 'Location where the archive should be saved',
            name: 'output_dir', 
            trim: true
        )
    }
                            
    environment {
        PYTHON_BIN = "/usr/cisco/bin/python3"
    }
    
    stages {        
        stage('Setup') {
            steps {
                sh '${PYTHON_BIN} -m venv .venv'
                sh '.venv/bin/pip install pyyaml'
            }
        }
        
        stage('Generate') {
            steps {
                sh '.venv/bin/python exec/utils/reporting/google_reporter.py ${firex_ids} google_logs'
                script {
                    def total_test_count = sh(
                        script: "grep -Rnw google_logs -e '<testsuite' | wc -l",
                        returnStdout: true
                    ).trim()
                    
                    def passing_test_count = sh(
                        script: "grep -Rnw google_logs -e 'failures=\"0\"' | wc -l",
                        returnStdout: true
                    ).trim()
                    
                    echo "Found  ${passing_test_count}/${total_test_count} passing tests"
                    currentBuild.description = "Passing ${passing_test_count}/${total_test_count} tests"
                    commit_hash = sh(
                        script: "grep -Rnw google_logs -e 'name=\"git.commit\"' | grep -oE '[0-9a-f]{40}' | head -n1 | cut -c 1-7",
                        returnStdout: true
                    ).trim()
                    
                    echo "Commit hash: ${commit_hash}"
                }
            }
        }
        
        stage('Compress') {
            steps {
                dir ("google_logs") {
                    script {
                        def now = new Date()
                        def timestamp = now.format("YYMMdd-HHmm", TimeZone.getTimeZone('America/Toronto'))
                        def fname = "CISCO.8808.${timestamp}.${commit_hash}.zip"

                        sh "zip -r ${fname} feature/*"
                        sh "cp ${fname} ${output_dir}/${fname}"
                        sh "rm -rf ${output_dir}/latest"
                        sh "mkdir -p ${output_dir}/latest"
                        sh "ln -s ${output_dir}/${fname} ${output_dir}/latest/${fname}"
                    }
                }
            }
        }
    }
}
