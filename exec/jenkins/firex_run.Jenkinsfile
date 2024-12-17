import groovy.transform.Field

@Field 
def image_path, image_lineup, image_efr, image_version

def testbeds, testbeds_override, testbeds_locked
def ts_internal, ts_absolute, ts_firex, ts_to_run
def firex_id

def auto_ws_dir = "/auto/b4ws/jenkins/workspace"
def unique_id = UUID.randomUUID().toString()
def auto_job_dir = "${auto_ws_dir}/${unique_id}"

def testsuite_filters_params = [
    'Must pass only', 'Test names', 'Test groups', 'Tests to exclude', 
    'Test groups to exclude', 'Test ordering'
]

def test_revision_params = ['Test branch', 'Test PR', 'Test commit hash']

def test_override_params = ['Test repository', 'Test args'] + test_revision_params

def forbidden_params_per_chain = [
    Nightly: ['Testbeds', 'Interactive Mode', 'Pause Run', 'Diff file', 'SMUs'] + testsuite_filters_params + test_override_params,
    CulpritFinder: ['Image Path'],
    B4FeatureCoverageRunTests: [],
    RunTests: []
]

def required_params_per_chain = [
    Nightly: [],
    CulpritFinder: ['Good EFR', 'Bad EFR', 'Build Image'],
    B4FeatureCoverageRunTests: ['Feature ID', 'Collect Coverage', 'Build Image'],
    RunTests: []
]

@Field
def firex_store_cache = [:]

pipeline {
    agent {
        node {
            label "${getNodeLabel()}" 
            customWorkspace "${auto_job_dir}"
        }
    }

    tools {
        go 'go1.21.3'
    }
    
    options {
        disableConcurrentBuilds()
    }
    
    parameters {
        separator(sectionHeader: "General")
        persistentString(name: 'Execution node label', defaultValue: 'ads', description: 'Label for Jenkins execution node. Defaults to "ads".', trim: true)
        persistentChoice(name: 'FireX Chain', choices: ['RunTests', 'Nightly', 'CulpritFinder', 'B4FeatureCoverageRunTests'], description: 'This should almost always be set to RunTests. Nightly should only be used with testsuites stored in the FireX testsuite store and never with private images. The result of Nightly runs will be exported to CosmosX.')
        persistentString(name: 'Testbeds', defaultValue: '', description: 'List of testbed IDs to use as defined in ./exec/testbeds.yaml. The testbeds specified here overrides the one specified in the YAML files for all testsuites.', trim: true)
        persistentString(name: 'Testsuites', defaultValue: 'exec/tests/v2/fp_published.yaml', description: 'List of testsuites. You can specify an internal YAML relative to the repo root, a FireX suite stored in the FireX testsuite store, or a full path to a private YAML file. Alternatively, you can provide a FireX run ID from which to extract the testsuite (prefix the run id with failed@ to extract only failed test). Note, the testsuite filters below will only work with an internal YAML.', trim: true)
        
        separator(sectionHeader: "Testsuite filters")
        persistentBoolean(name: 'Must pass only', defaultValue: false, description: 'Only include tests marked as must pass.')
        persistentString(name: 'Test names', defaultValue: '', description: 'Test names to include (e.g., gNOI-5.1,gNOI-5.2).', trim: true)
        persistentString(name: 'Test groups', defaultValue: '', description: 'Test group names to include (e.g., ancx).', trim: true)
        persistentString(name: 'Tests to exclude', defaultValue: '', description: 'Test names to excludes (e.g., gNOI-3.1).', trim: true)
        persistentString(name: 'Test groups to exclude', defaultValue: '', description: 'Test group names to exclude (e.g., ancx).', trim: true)
        persistentChoice(name: 'Test ordering', choices: ['', 'By Priority', 'Randomize'], description: '')

        separator(sectionHeader: "Test Overrides")
        persistentChoice(name: 'Test repository', choices: ['', 'Internal', 'Public'], description: 'Repository to pull the test from. "Internal" refers to B4Test/featureprofiles whereas "Public" refers to openconfig/featureprofiles.')
        persistentString(name: 'Test branch', defaultValue: '', description: 'Run the test from the specified branch', trim: true)
        persistentString(name: 'Test PR', defaultValue: '', description: 'Run the test from the specified PR', trim: true)
        persistentString(name: 'Test commit hash', defaultValue: '', description: 'Run the test from the specified commit hash', trim: true)
        persistentText(name: 'Test args', defaultValue: '', description: 'List of test args, one per line')
        persistentText(name: 'Test env', defaultValue: '', description: 'List of test env, one per line in "key: value" format')

        separator(sectionHeader: "Test Execution")
        persistentBoolean(name: 'Verbose Mode', defaultValue: true, description: 'Run test in verbose mode (i.e., -v 5 -alsologtostderr)')
        persistentBoolean(name: 'Collect DUT Info', defaultValue: true, description: 'Allow Ondatra to collect DUT information (i.e., collect_dut_info=true)')
        persistentBoolean(name: 'Interactive Mode', defaultValue: false, description: 'Run test in interactive mode. This option can only be used when specifying exactly one test under "Test names". When selected, FireX will not execute the test. It will drop to a shell allowing the user to use the dlv debugger to manually execute the test. This option is also useful to boot up a SIM for manual testing.')
        
        separator(sectionHeader: "On Test Failure")
        persistentBoolean(name: 'Collect Debug Files', defaultValue: false, description: 'Collect showtechs and other debug files on test failure. Note that this option can significantly increase the test runtime. The script used for debug files collection can be found here: exec/utils/debug/collect_debug_files_test.go.')
        persistentBoolean(name: 'Pause Run', defaultValue: false, description: 'Pause run on test failure. The run will be paused if the test fails. A test in this context is an entire FireX suite or go package (e.g., gNOI-5.1).')
    
        separator(sectionHeader: "Image Info")
        persistentBoolean(name: 'Install Image', defaultValue: false, description: 'Install the specified image. The image installation script can be found here: exec/utils/software_upgrade/software_upgrade_test.go')
        persistentBoolean(name: 'Force Install Image', defaultValue: false, description: 'Force install the image even if it is already installed. This option might be necessary if the image has additional SMUs and an EFR that matches the installed image.')
        persistentString(name: 'Image Path', defaultValue: '', description: '', trim: true)

        separator(sectionHeader: "Build Image")
        persistentBoolean(name: 'Build Image', defaultValue: false, description: 'If selected, the image will be built from the lineup and EFR specified below. Otherwise, the pipeline will attempt to find the latest image for the lineup using PIMS.')
        persistentString(name: 'Lineup', defaultValue: '', description: 'Build the image from the specified lineup. Defaults to "xr-dev".', trim: true)
        persistentString(name: 'EFR', defaultValue: '', description: 'Build the specified EFR. Defaults to "LATEST".', trim: true)
        persistentString(name: 'Diff file', defaultValue: '', description: 'Optional path to a diff file to apply before building the image', trim: true)

        separator(sectionHeader: "SMUs")
        persistentText(name: 'SMUs', defaultValue: '', description: 'List of SMUs to install, one per line. The SMU install script can be found here: exec/utils/smu_install/smu_install_test.go')

        separator(sectionHeader: "SIM Options")
        persistentBoolean(name: 'Setup MTLS', defaultValue: false, description: '')

        separator(sectionHeader: "Code Coverage")
        persistentBoolean(name: 'Collect Coverage', defaultValue: false, description: '')
        persistentBoolean(name: 'Use SSH For Coverage Collection', defaultValue: true, description: 'This option is required if running on hardware or spitfire_d SIMs.')
        persistentString(name: 'Feature ID', defaultValue: '', description: 'Feature ID to use for pulling diff files.', trim: true)
        persistentString(name: 'XR Components', defaultValue: '', description: 'List of XR components to instrument. Only needed if the Feature ID cannot be used.', trim: true)

        separator(sectionHeader: "CulpritFinder")
        persistentString(name: 'Good EFR', defaultValue: '', description: '', trim: true)
        persistentString(name: 'Bad EFR', defaultValue: '', description: '', trim: true)

        separator(sectionHeader: "Reporting")
        persistentString(name: 'Email CC', defaultValue: '', description: '', trim: true)
        persistentString(name: 'Run Reason', defaultValue: '', description: '', trim: true)

        separator(sectionHeader: "Other")
        persistentBoolean(name: 'Decomission testbeds', defaultValue: false, description: 'Decomission testbeds after each test. This option makes sure the TB is "recycled" between each test. For sim runs, this ensures that a new sim is brought up for each test.')
        persistentString(name: 'Number of FireX workers', defaultValue: '', description: 'The number of FireX workers to launch. This is the number of tests that can execute in parallel (subject to testbed availability). Defaults to the number of testbeds.', trim: true)
        persistentString(name: 'Extra FireX Args', defaultValue: '', description: '', trim: true)
        
    }

    stages {        
        stage('Validate') {
            steps {
                script {
                    for(n in forbidden_params_per_chain[params['FireX Chain']]) {
                        if(params[n]) {
                            error "Parameter '${n}' is not allowed for '${params['FireX Chain']}' chain"
                        }
                    }

                    for(n in required_params_per_chain[params['FireX Chain']]) {
                        if(!params[n]) {
                            error "Parameter '${n}' is required for '${params['FireX Chain']}' chain"
                        }
                    }

                    testbeds = parseTestbeds(params['Testbeds'])
                    
                    testsuite_param = params['Testsuites']
                    if(!testsuite_param && isRebuild()) {
                        def upstream_firex_id = getUpstreamFireXID()
                        if (upstream_firex_id) {
                            testsuite_param = "failed@${upstream_firex_id}"
                        } 
                    }
                    (ts_internal, ts_absolute, ts_firex) = parseTestsuites(testsuite_param)
                    ts_to_run = [] + ts_absolute + ts_firex

                    // no testsuites to run
                    if((ts_internal + ts_absolute + ts_firex).size() == 0) {
                        error "No testsuites found."
                    }

                    // cannot mix testsuite types
                    if(ts_internal.size() != 0 && (ts_absolute + ts_firex).size() != 0) {
                        error "Mixing internal testsuites with FireX testsuites is not supported"
                    }

                    if(ts_internal.size() == 0) {
                        for(n in testsuite_filters_params) {
                            if(params[n]) {
                                error "Parameter '${n}' is only supported when using internal testsuite YAML."
                            }
                        }

                        if(testbeds.size() > 0) {
                            testbeds_override = true
                        }
                    }

                    
                    if(test_revision_params.count {params[it]} > 1) {
                        error "Ony one of 'Test branch', 'Test PR', or 'Test revision' can be specified"
                    }

                    if((ts_internal + ts_absolute + ts_firex).size() != 1) {
                        if(params['Interactive Mode']) {
                            error "Exactly one testsuite must be specified for Interactive Mode."
                        }

                        if(params['FireX Chain'] == 'CulpritFinder') {
                            error "Exactly one testsuite must be specified when using 'CulpritFinder' Chain."
                        }
                    }

                    if((ts_internal + ts_absolute).size() != 0) {
                        if(params['FireX Chain'] == 'Nightly') {
                            error "Only testsuites registered in the FireX testsuite store can be used with the Nightly chain."
                        }
                    }

                    if(params['Number of FireX workers']) {
                        if(!params['Number of FireX workers'].isNumber()) {
                            error "'Number of FireX workers' must be an integer"
                        }
                    }

                    if(params['Image Path']) {
                        if(params['Build Image']) {
                            error "Must not specify 'Image Path' when 'Build Image' is selected."
                        }

                        if(!fileExists("${params['Image Path']}")) {
                            error "Image ${params['Image Path']} does not exist."
                        }
                    }

                    if(params['Code Coverage'] && params['Build Image']) {
                        if(!params['Feature ID'] && !params['XR Components']) {
                            error "Must specify either 'Feature ID' or 'XR Components' for code coverage."
                        }
                    }
                }
            }
        }
        
        stage('Find Image') {
            when {
                expression {
                    !params['Build Image'] && !params['Image Path']
                 }
            }

            steps {
                script {
                    image_path = getLatestImage(lineup: getLineup())
                    echo "Found image ${image_path}"
                }
            }
        }
        
        stage('Image Info') {
            when {
                expression {
                    !params['Build Image']
                }
            }

            steps {
                script {
                    (image_path, image_lineup, image_efr, image_version) = getImageInfo(getImage())
                    echo "Image ${image_path} has lineup ${image_lineup}, efr ${image_efr}, and version ${image_version}"
                }
            }
        }
        
        stage('Generate Testsuite') {
            when {
                expression {
                    ts_internal.size() > 0
                }
            }

            steps {
                script {
                    testgen_cmd_parts = [
                        "go run exec/firex/v2/testsuite_generator.go",
                        "-files ${ts_internal.join(',')}",
                        "-internal_repo_rev '${env.GIT_COMMIT}'"
                    ]

                    if(testbeds.size() > 0) {
                        testgen_cmd_parts.add("-testbeds ${testbeds.join(",")}")
                    }

                    if(params['Must pass only']) {
                        testgen_cmd_parts.add('-must_pass_only')
                    }

                    if(params['Test ordering'] == 'By Priority') {
                        testgen_cmd_parts.add('-sort')
                    }
                    else if(params['Test ordering'] == 'Randomize') {
                        testgen_cmd_parts.add('-randomize')
                    }

                    if(params['Test names']) {
                        testgen_cmd_parts.add("-test_names ${params['Test names']}")
                    }

                    if(params['Tests to exclude']) {
                        testgen_cmd_parts.add("-exclude_test_names ${params['Tests to exclude']}")
                    }

                    if(params['Test groups']) {
                        testgen_cmd_parts.add("-group_names ${params['Test groups']}")
                    }

                    if(params['Test groups to exclude']) {
                        testgen_cmd_parts.add("-exclude_group_names ${params['Test groups to exclude']}")
                    }

                    def test_rev_spec = ""
                    if(params['Test revision']) {
                        test_rev_spec += "REV#" + params['Test revision']
                    }
                    else if(params['Test branch']) {
                        test_rev_spec += "BR#" + params['Test branch']
                    }
                    else if(params['Test PR']) {
                        test_rev_spec += "PR#" + params['Test PR']
                    }

                    if(test_rev_spec) {
                        if(params['Test repository'] == 'Internal') {
                            test_rev_spec = "I-" + test_rev_spec
                        }

                        testgen_cmd_parts.add("-test_repo_rev '${test_rev_spec}'")
                    }

                    if(params['Test args']) {
                        def args_list = getTestArgs()
                        if(args_list.size() > 0) {
                            testgen_cmd_parts.add("--test_args '${args_list.join(',')}'")
                        }
                    }

                    if(params['Test env']) {
                        def env_list = getTestEnv()
                        if(env_list.size() > 0) {
                            testgen_cmd_parts.add("--env '${env_list.join(',')}'")
                        }
                    }

                    if(params['Interactive Mode']) {
                        testgen_cmd_parts.add("-use_short_names")
                    }

                    testgen_cmd = testgen_cmd_parts.join(' ')
                    sh  "${testgen_cmd} > testsuite.yaml"
                    sh  "cat testsuite.yaml"
                    ts_to_run.add("testsuite.yaml")
                }
            }
        }

        stage('Lock Testbeds') {
            steps {
                script {
                    if(testbeds.size() == 0) {
                        testbeds = extractTestbedsFromSuites(ts_to_run)
                    }
                    
                    if(testbeds.size() == 0) {
                        error "Could not find any testbeds."
                    }
                    
                    lockTestbeds(testbeds)
                    testbeds_locked = true
                }
            }
        }

        stage('Start') {
            failFast true
            parallel {
                stage('Testing') {
                    steps {
                        script {
                            def firex_chain = params['FireX Chain']
                            def firex_plugins = [
                                "${env.WORKSPACE}/exec/firex/v2/runner.py"
                            ]
                            if(firex_chain == 'B4FeatureCoverageRunTests') {
                                firex_plugins.add("${env.WORKSPACE}/feature_coverage.py")
                            } else if(firex_chain != 'CulpritFinder') {
                                firex_plugins.add("webdt_cit.py")
                            }

                            def decomission_testbeds = (params['Decomission testbeds'] || testbeds.size() > 1) ? 1 : 0
                            def num_workers = params['Number of FireX workers'] ? params['Number of FireX workers'] : testbeds.size()

                            firex_cmd_parts = [
                                "B4_FIREX_TESTBEDS_COUNT=${num_workers}",
                                "B4_FIREX_DECOMMISSION_TESTBED=${decomission_testbeds}",
                                "/auto/firex/bin/firex", 
                                "--chain ${firex_chain}",
                                "--plugins ${firex_plugins.join(',')}", 
                                "--lineup ${getLineup()}",
                                "--tag ${getEFR()}",
                                "--internal_fp_repo_rev ${env.GIT_COMMIT}",
                                "--sync"
                            ]

                            if(firex_chain == "CulpritFinder") {
                                firex_cmd_parts.add("--testsuite ${ts_to_run.join(',')}")
                                firex_cmd_parts.add("--good_efr ${params['Good EFR']}")
                                firex_cmd_parts.add("--bad_efr ${params['Bad EFR']}")
                                firex_cmd_parts.add("--use_culprit_finder_cache False")
                            } else {
                                firex_cmd_parts.add("--testsuites ${ts_to_run.join(',')}")
                            }

                            if(getImage()) {
                                firex_cmd_parts.add("--images ${getImage()}")
                            }

                            if(params['Collect Coverage']) {
                                firex_cmd_parts.add("--cflow true")
                                if(params['Use SSH For Coverage Collection']) {
                                    firex_cmd_parts.add("--cflow_over_ssh true")
                                }
                            }

                            if(params['Build Image']) {
                                if(params['Diff file']) {
                                    firex_cmd_parts.add("--diff_file ${params['Diff file']}")
                                }

                                if(params['Feature ID']) {
                                    firex_cmd_parts.add("--feature_id ${params['Feature ID']}")
                                }

                                if(params['XR Components']) {
                                    firex_cmd_parts.add("--comps ${params['XR Components']}")
                                }
                            }

                            firex_cmd_parts.add("--collect_debug_files ${params['Collect Debug Files']}")
                            firex_cmd_parts.add("--collect_dut_info ${params['Collect DUT Info']}")
                            firex_cmd_parts.add("--test_verbose ${params['Verbose Mode']}")
                            
                            
                            if(firex_chain == "CulpritFinder" || params['Force Install Image']) {
                                firex_cmd_parts.add("--install_image true")
                                firex_cmd_parts.add("--force_install true")
                            } else if(params['Build Image']) {
                                firex_cmd_parts.add("--install_image true")
                            } else {
                                firex_cmd_parts.add("--install_image ${params['Install Image']}")
                            }

                            if(params['SMUs']) {
                                def smu_list = getSMUs()
                                firex_cmd_parts.add("--smus '${smu_list.join(',')}'")
                            }

                            if(params['Setup MTLS']) {
                                firex_cmd_parts.add("--sim_use_mtls true")
                            }

                            if(params['Interactive Mode']) {
                                firex_cmd_parts.add("--debug_suites '${params['Test names']}'")
                                firex_cmd_parts.add("--test_debug true")
                            }

                            if(params['Pause Run']) {
                                firex_cmd_parts.add("--pause_if_tests_fail true")
                            }

                            if(ts_internal.size() == 0) { // no need for internal testsuites
                                if(params['Test repository']) {
                                    firex_cmd_parts.add("--internal_test ${params['Test repository'] == 'Internal'}")
                                }
                                if(params['Test revision']) {
                                    firex_cmd_parts.add("--test_revision ${params['Test revision']}")
                                }
                                else if(params['Test branch']) {
                                    firex_cmd_parts.add("--test_branch ${params['Test branch']}")
                                }
                                else if(params['Test PR']) {
                                    firex_cmd_parts.add("--test_pr ${params['Test PR']}")
                                }

                                if(params['Test args']) {
                                    def args_list = getTestArgs()
                                    if(args_list.size() > 0) {
                                        firex_cmd_parts.add("--test_args '${args_list.join(',')}'")
                                    }
                                }

                                if(params['Test env']) {
                                    def env_list = getTestEnv()
                                    if(env_list.size() > 0) {
                                        firex_cmd_parts.add("--env '${env_list.join(',')}'")
                                    }
                                }

                                if(testbeds_override) {
                                    firex_cmd_parts.add("--testbeds ${testbeds.join(',')}")
                                }
                            }

                            if(params['Email CC']) {
                                firex_cmd_parts.add("--cc ${params['Email CC']}")
                            }

                            if(params['Run Reason']) {
                                firex_cmd_parts.add("--reason '${params['Run Reason']}'")
                            }

                            firex_cmd = firex_cmd_parts.join(' ')
                            firex_cmd += ' ' + params['Extra FireX Args']
                            sh "${firex_cmd}"   
                        }
                    }
                }

                stage('Monitoring') {
                    steps {
                        script {
                            while(true) {
                                def matcher = manager.getLogMatcher(".*\\sFireX\\sID:\\s(.*)")
                                if(matcher?.matches()) {
                                    firex_id = matcher.group(1).trim()
                                    matcher = null
                                    
                                    def firex_tracker = "https://firex-north.cisco.com/test_tracker/#/${firex_id}"
                                    currentBuild.description = "<a target='_blank' href='${firex_tracker}'>${firex_id}</a>"
                                    break
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    post {
        always {
            script {
                withEnv(['JENKINS_NODE_COOKIE=dontkill']) {
                    if(testbeds_locked) {
                        releaseTestbeds(testbeds)
                    }
                }
            }

            cleanWs(cleanWhenNotBuilt: true,
                    deleteDirs: true,
                    disableDeferredWipeout: true,
                    notFailBuild: true)
            
            deleteDir()

            dir("${workspace}@tmp") {
                deleteDir()
            }
            
            dir("${workspace}@script") {
                deleteDir()
            }
        }

        aborted {
            script {
                if(firex_id) {
                    sh "/auto/firex/bin/firex kill --force --uid ${firex_id} || 1"
                }
            }
        }
    }
}

def getNodeLabel() {
    return params['Execution node label'] != "" ? params['Execution node label'] : 'ads'
}

def getLineup() {
    if(image_lineup) {
        return image_lineup
    }
    return params['Lineup'] != "" ? params['Lineup'] : 'xr-dev'
}

def getEFR() {
    if(image_efr) {
        return image_efr
    }
    return params['EFR'] != "" ? params['EFR'] : 'LATEST'
}

def getImage() {
    if(image_path) {
        return image_path
    }
    return params['Image Path']
}

def getTestArgs() {
    def args_list = []
    for(l in params['Test args'].split('\n')) {
        sp = l.trim().replaceAll("'", '"')
        if(sp) args_list.add(sp)
    }
    return args_list
}

def getTestEnv() {
    def env_list = []
    for(l in params['Test env'].split('\n')) {
        sp = l.trim().replaceAll("'", '"')
        if(sp) env_list.add(sp)
    }
    return env_list
}

def getSMUs() {
    def smus_list = []
    for(l in params['SMUs'].split('\n')) {
        sp = l.trim()
        if(sp) {
            if(!fileExists("${sp}")) {
                error "SMU ${s} does not exist."
            }
            smus_list.add(sp)
        }
    }
    return smus_list
}


// Testbed utils
def parseTestbeds(String testbeds_list) {
    def testbeds = []
    for(tb in testbeds_list.split(',')) {
        id = tb.trim()
        if(id) testbeds.add(id)
    } 

    Map m = readYaml(file: "exec/testbeds.yaml")["testbeds"]
    for(id in testbeds){
        if(!m.containsKey(id)) {
            error "Testbed ${id} not found"
        }
    }
    return testbeds
}

def lockTestbeds(List testbeds) {
    sh "/auto/tftpboot-ottawa/b4/bin/tblock lock -w -r '${env.JOB_BASE_NAME}' '${testbeds.join(',')}'"
}

def releaseTestbeds(List testbeds) {
    sh "/auto/tftpboot-ottawa/b4/bin/tblock release '${testbeds.join(',')}'"
}

// Image utils
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

def getLatestImage(Map params) {
    def dev_image = ""
    if(params.dev_image) {
        dev_image = "--dev"
    }

    if(!dev_image && params.lineup in ['xr-dev', 'xr-dev.lu']) {
        return sh(
            script: "realpath /auto/b4ws/xr/builds/nightly/latest/img-8000/8000-x64.iso",
            returnStdout: true
        ).trim()
    }

    def imageDetails = sh(
        script: "python3 exec/utils/pims/pims.py --lineup ${params.lineup} ${dev_image}",
        returnStdout: true
    ).trim()

    def tokens = imageDetails.split( ',' )
    if(tokens.length > 0) {
        return tokens[0]
    }

    error "Could not find image"
}

// Testsuite utils
def getFirexStore() {
    return "/auto/firex/PRODUCTION/firex_configs/testsuites_store"
}

def findInFirexStore(String testsuite) {
    if (firex_store_cache.containsKey(testsuite)) {
        return firex_store_cache[testsuite]
    }

    def files = sh(
        script: "grep -Ril '${testsuite}' ${getFirexStore()}",
        returnStdout: true
    ).trim().split('\n')

    def result = files.size() > 0 ? files[0] : ""
    firex_store_cache[testsuite] = result
    return result
}

def generateSuiteFromRun(String firex_id, boolean failed_only = false) {
    def out_file = "${env.WORKSPACE}/${firex_id}.yaml"
    def failed_only_arg = failed_only ? "--failed_only" : ""
    try {
        sh "python3 exec/utils/firex/generate_failed_suite.py ${firex_id} ${out_file} ${failed_only_arg}"
        return out_file
    } catch (Exception e) {
        return null
    }
}

def parseTestsuites(String testsuites_list) {
    def suites = []
    for(s in testsuites_list.split(',')) {
        n = s.trim()
        if(n) suites.add(n)
    } 

    def ts_internal = []
    def ts_absolute = []
    def ts_firex = []

    for(f in suites){
        if(f ==~ /(failed@)?FireX-\w+-\d{6}-\d{6}-\d{5}/) {
            def failed_only = f.startsWith("failed@")
            f = f.replace("failed@", "")
            def suite = generateSuiteFromRun(f, failed_only) 
            if(suite) { 
                ts_absolute.add(suite)
            } else {
                error "Failed to generate testsuite suite from ${f}"
            }
        } else if(f.startsWith("/") && fileExists(f)) {
            ts_absolute.add(f)
        } else if(fileExists(f)) {
            ts_internal.add(f)
        } else if(fileExists("${getFirexStore()}/${f}")) {
            ts_firex.add(f)
        } else {
            if(findInFirexStore(f)) {
                ts_firex.add(f)
            } else {
                error "Testsuite ${f} does not exist."
            }
        }
    }

    return [ts_internal, ts_absolute, ts_firex]
}

def extractTestbedsFromSuites(List testsuites) {
    def testbeds = []

    for(f in testsuites){
        def path, is_file

        if(fileExists(f)) {
            path = f
            is_file = true
        } else if(fileExists("${getFirexStore()}/${f}")) {
            path = "${getFirexStore()}/${f}"
            is_file = true
        } else {
            path = findInFirexStore(f)
            is_file = false
        }

        if(!path) {
            error "Error reading testsuite ${f}"
        }

        Map m = readYaml(file: path)
        
        if(is_file) {
            for(e in m) {
                for(tb in e.value['testbeds']) {
                    testbeds.add(tb)
                }

                for(spe in e.value['script_paths']) {
                    for(sp in spe) {
                        for(tb in sp.value['testbeds']) {
                            testbeds.add(tb)
                        }
                    }
                }
            }
        } else {
            for(e in m) {
                def script
                
                out:
                for(spe in e.value['script_paths']) {
                    for(sp in spe) {
                        if(sp.key == f) {
                            script = sp.value
                            break out
                        }
                    }
                }

                if(!script) {
                    continue
                }

                if(script.containsKey('testbeds')) {
                    for(tb in script['testbeds']) {
                        testbeds.add(tb)
                    }
                } else {
                    for(tb in e.value['testbeds']) {
                        testbeds.add(tb)
                    }
                }
            }
        }
    }

    return testbeds.unique()
}

def getUpstreamFireXID() {
    for(b in currentBuild.getUpstreamBuilds()) {
        def desc = b.getDescription()
        def matcher = desc =~ /FireX-\w+-\d{6}-\d{6}-\d{5}/
        if (matcher) {
            return matcher[0]
        }
        break
    }
    return null
}

def isRebuild() {
    def causes = currentBuild.getBuildCauses()
    return causes.any { it._class == 'com.sonyericsson.rebuild.RebuildCause' }
}
