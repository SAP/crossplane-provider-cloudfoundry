import groovy.json.JsonOutput

void call(Map params) {
    echo "Start - Extension for stage: ${params.stageName}"
    def script = params?.script

    def targetRegistryURL = script.commonPipelineEnvironment.configuration.general.crossplane.targetOCIRegistry

    crossplaneProviderPrepareBuild script: script, targetRegistryUrl: targetRegistryURL

    params.originalStage()

    readPipelineEnv script: script

    def images = dockerPullFromStagingRegistry script: script, images: ["crossplane/provider-cloudfoundry", "crossplane/provider-cloudfoundry-controller"], targetRegistryUrl: targetRegistryURL

    def imagesJson = JsonOutput.toJson(images)

    withCredentials([
                    usernamePassword(credentialsId: 'hyperspace-github-tools-sap-co-hyperspace-serviceuser', usernameVariable: 'GIT_USERNAME', passwordVariable: 'GH_TOKEN'),
                    string(credentialsId: 'cf_environment', variable: 'CF_ENVIRONMENT'),
                    string(credentialsId: 'cf_credentials', variable: 'CF_CREDENTIALS'),
                    usernamePassword(credentialsId: 'common-repository-serviceuser', usernameVariable: 'ARTIFACTORY_USERNAME', passwordVariable: 'ARTIFACTORY_PASSWORD'),
                ]) {

        env.CF_ENVIRONMENT = CF_ENVIRONMENT
        env.CF_CREDENTIALS = CF_CREDENTIALS
        env.ARTIFACTORY_USERNAME = ARTIFACTORY_USERNAME
        env.ARTIFACTORY_PASSWORD = ARTIFACTORY_PASSWORD
        env.E2E_IMAGES = imagesJson

        lock(resource: "${env.JOB_NAME}/kind", inversePrecedence: true) {
            kindExecute(script: this) {
                sh '''
                    env
                    env

                    export PATH=/usr/local/go/bin:$PATH
                    go env -w GOPRIVATE=github.tools.sap
                    git config --global url."https://${GIT_USERNAME}:${GH_TOKEN}@github.tools.sap".insteadOf "https://github.tools.sap"

                    go install gotest.tools/gotestsum@latest
                    ~/go/bin/gotestsum --debug --format standard-verbose --junitfile=integration-tests.xml -- --tags=e2e ./.../e2e -test.v -test.short -short -v
                '''
            }
        }
    }

    testsPublishResults script: script, archive: true, junit: [pattern: '**//* integration-tests.xml', allowEmptyResult: false, archive: true,]

    sapCumulusUpload script: script, filePattern: '**//* integration-tests.xml', stepResultType: 'integration-test'


}
return this
