def global_parameters = []
def image_path, image_version, image_lineup, image_efr

pipeline {
    agent { label "ads" }

    parameters {
        persistentString(name: 'Boot Image', defaultValue: '', description: 'The boot image used to bring up the testbeds. On hardware, the starting image will be installed using "install replace reimage".', trim: true)
        persistentString(name: 'Upgrade Image', defaultValue: '', description: 'The target image to install.', trim: true)
        persistentString(name: 'Unsupported Image', defaultValue: '', description: 'Optional unsupported image path for negative tests.', trim: true)
        persistentString(name: 'Jenkins job path', defaultValue: '', description: 'Jenkins job path for the install tests pipeline.', trim: true)
    }

    stages {
        stage('Validate') {
            steps {
                script {
                    for(p in ['Boot Image', 'Upgrade Image', 'Jenkins job path']) {
                        if(!params[p]) {
                            error "Parameter '${p}' is required."
                        }
                    }
                    
                    for(p in ['Boot Image', 'Upgrade Image', 'Unsupported Image']) {
                        if(params[p] && !fileExists(params[p])) {
                            error "Image ${params[p]} does not exist."
                        }
                    }
                }
            }
        }
        
        stage('Image Info') {
            steps {
                script {
                    (image_path, image_lineup, image_efr, image_version) = getImageInfo(params['Upgrade Image'])
    
                    global_parameters += [
                        string(name: 'Image Path', value: "${params['Boot Image']}"),
                        booleanParam(name: 'Install Image', value: true)
                    ]
                }
            }
        }
        
        stage('Trigger Install') {
            steps {
                script {
                    def test_env = [
                        "UPGRADE_IMAGE_PATH: ${image_path}", 
                        "UPGRADE_IMAGE_VERSION: ${image_version}", 
                        "UPGRADE_IMAGE_LINEUP: ${image_lineup}", 
                        "UPGRADE_IMAGE_EFR: ${image_efr}", 
                        "BOOT_IMAGE_PATH: ${params['Boot Image']}", 
                    ]

                    if(params['Unsupported Image']) {
                        test_env += [ "UNSUPPORTED_IMAGE_PATH: ${params['Unsupported Image']}" ]
                    }

                    build job: params['Jenkins job path'], parameters: [
                        string(name: 'Test env', value: test_env.join('\n'))
                    ] + global_parameters, wait: true
                }
            }
        }
    }
}

def getImageInfo(String imagePath) {
    def imageInfo = sh(
        script: "isoinfo -R -x /mdata/build-info.txt -i ${imagePath}",
        returnStdout: true
    ).trim()
    
    def lineup, efr, version
    for(line in imageInfo.split('\n')) {
        if(line.startsWith("Lineup")) {
            def (k,v) = line.split('=')
            def (l,e) = v.split('%')
            lineup = l.trim()
            efr = e.trim()
        }
        else if(line.startsWith("XR version")) {
            def (k,v) = line.split('=')
            version = v.split('-')[0].trim()
        }
    }
    return [imagePath, lineup, efr, version]
}
