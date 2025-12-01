def global_parameters = []
def image_path, image_version, image_full_version, image_lineup, image_efr

pipeline {
    agent { label "ads" }

    parameters {
        persistentString(name: 'Boot Image', defaultValue: '', description: 'The boot image used to bring up the testbeds. On hardware, the starting image will be installed using "install replace reimage".', trim: true)
        persistentText(name: 'Upgrade Path', defaultValue: '', description: 'Newline-separated list of target images to install sequentially.', trim: true)
        persistentString(name: 'Jenkins job path', defaultValue: '', description: 'Jenkins job path for the install tests pipeline.', trim: true)
    }

    stages {
        stage('Validate') {
            steps {
                script {
                    for(p in ['Upgrade Path', 'Jenkins job path']) {
                        if(!params[p]) {
                            error "Parameter '${p}' is required."
                        }
                    }
                    
                    if(params['Boot Image'] && !fileExists(params['Boot Image'])) {
                        error "Boot image ${params['Boot Image']} does not exist."
                    }
                    
                    def upgradeImagePaths = params['Upgrade Path'].split('\n').collect { it.trim() }.findAll { it }
                    for(imgPath in upgradeImagePaths) {
                        if(!fileExists(imgPath)) {
                            error "Upgrade image ${imgPath} does not exist."
                        }
                    }
                }
            }
        }
        
        stage('Image Info') {
            steps {
                script {
                    def upgradeImagePaths = params['Upgrade Path'].split('\n').collect { it.trim() }.findAll { it }
                    def upgradeImageList = []
                    def upgradeVersions = []
                    
                    for(imgPath in upgradeImagePaths) {
                        def (path, lineup, efr, version, fullVersion) = getImageInfo(imgPath)
                        upgradeImageList.add("${path}:${version}")
                        upgradeVersions.add(version)
                    }
                    
                    def upgradeImageListEnv = upgradeImageList.join(',')
                    
                    // Build the upgrade path string for run reason
                    def upgradePath = ""
                    if(params['Boot Image']) {
                        def (bootPath, bootLineup, bootEfr, bootVersion, bootFullVersion) = getImageInfo(params['Boot Image'])
                        upgradePath = bootVersion
                        global_parameters += [
                            string(name: 'Image Path', value: "${params['Boot Image']}"),
                            booleanParam(name: 'Install Image', value: true)
                        ]
                    } else {
                        global_parameters += [
                            string(name: 'Image Path', value: ""),
                            booleanParam(name: 'Install Image', value: false)
                        ]
                    }
                    
                    for(ver in upgradeVersions) {
                        upgradePath += "->${ver}"
                    }
                }
            }
        }
        
        stage('Trigger Install') {
            steps {
                script {
                    def test_env = [
                        "UPGRADE_IMAGE_LIST: ${upgradeImageListEnv}", 
                    ]

                    build job: params['Jenkins job path'], parameters: [
                        string(name: 'Test env', value: test_env.join('\n')),
                        string(name: 'Run Reason', value: "BootZ Install Path ${upgradePath}")
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
    
    def lineup, efr, version, fullVersion, label
    for(line in imageInfo.split('\n')) {
        if(line.startsWith("Lineup")) {
            def (k,v) = line.split('=')
            def (l,e) = v.split('%')
            lineup = l.trim()
            efr = e.trim()
        }
        else if(line.startsWith("XR version")) {
            def (k,v) = line.split('=')
            version = v.replaceAll('-LNT$', '').trim()
        }
        else if(line.startsWith("GISO build command")) {
            def m = (line =~ /--label\s+([^\s]+)/)
            if(m.find()) {
                label = m.group(1).trim()
            }
        }
    }
    if(label) {
        fullVersion = version ? "${version}-${label}" : label
    } else {
        fullVersion = version
    }
    return [imagePath, lineup, efr, version, fullVersion]
}
