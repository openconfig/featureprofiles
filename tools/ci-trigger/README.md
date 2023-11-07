# ci-trigger

CI Trigger is a Google Cloud Run container that manages FeatureProfiles CI events.  The Cloud Run container uses the GitHub API to inspect pull requests and identify changes.  If a pull request changes Ondatra tests, an authorized user can comment in the pull request to cause CI Trigger to launch a Google Cloud Build task to validate tests on various virtual/hardware platforms.

## Design

CI Trigger responds to 3 types of events:

* GitHub WebHook - Pull Requests
* GitHub WebHook - Issue Comments
* Cloud PubSub - Badge Updates

On a [Pull Request](https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request) `opened` (new) or `synchronize` (updated) event, CI Trigger will fetch the git branch and inspect changes between the base and head branches. If there are any changed files in an Ondatra test directory, the test is marked as modified. Badge icons are initialized for the commit ID into Cloud Storage and a comment is posted to the pull request containing a summary of all the changes. Virtual tests are automatically launched if the PR author is authorized to run tests.

On an [Issue Comment](https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#issue_comment) `created` event, CI Trigger will check if the comment was made by an authorized user and contains a keyword to launch tests in a pull request. A job will be created for each device type requested to launch tests. Virtual tests are executed using Cloud Build, while physical tests are sent via pubsub message to another execution system. Badge status icons will be updated to mark that the test has been launched.

On a PubSub topic, CI Trigger listens for test status updates coming from Cloud Build tests.  Badge icons are updated based on the messages received.

A pull request is expected to traverse through these status codes:

| State | Description |
| - | - |
| pending authorization | An authorized user must comment in the pull request to start the process. |
| setup | A Cloud Build job has been created for the pull request. |
| pending execution | The Cloud Build job has started. |
| environment setup | KNE topology is being configured. |
| running | Ondatra test is running. |
| success or failure | Test has completed. |

## Setup

The installation is a one-time process and these steps are documented for reference.

### GitHub

#### Repository Configuration

In the Featureprofiles repository settings, create a [webhook](https://github.com/openconfig/featureprofiles/settings/hooks).  The secret will be configured in Cloud Secrets as the Webhook Secret.  Use "Let me select individual events" and select "Issue Comments" and "Pull Requests".  Unselect other events like "Pushes".

#### Robot User Configuration

Create a [fine-grained personal access token](https://github.com/settings/tokens?type=beta) on the GitHub bot account.  Set the Resource owner to "openconfig" and select the "featureprofiles" repository.  The user needs Read and Write access to "Pull Requests".  It also needs Read-Only access to organization "Members".  The access token generated will be used in Cloud Secrets as the API Secret.

### Google Cloud

#### Cloud Object Storage

A bucket for badge icons is required and needs to be world-readable.  This bucket will contain only small SVG objects.

```
gcloud storage buckets create gs://featureprofiles-ci --project=disco-idea-817 --default-storage-class=STANDARD --location=US --uniform-bucket-level-access
```

A second bucket is used by Cloud Build for running tests.  The objects in this bucket are the contents of the featureprofiles git repository checked out at the pull request commit.  Because this bucket could grow large over time, an expiration policy is used on objects.  Note that this bucket may already exist because Cloud Build creates it by default.

```
gcloud storage buckets create gs://disco-idea-817_cloudbuild --project=disco-idea-817 --default-storage-class=STANDARD --location=US --uniform-bucket-level-access
```

#### Cloud PubSub Topic

```
gcloud pubsub topics create featureprofiles-badge-status
gcloud pubsub subscriptions create featureprofiles-badge-status --topic featureprofiles-badge-status
gcloud pubsub topics add-iam-policy-binding featureprofiles-badge-status --member="serviceAccount:serviceAccountName@developer.gserviceaccount.com" --role="roles/pubsub.publisher"
```

#### Cloud Run Deployment

An Artifact Repository is required for the container image:

```
gcloud artifacts repositories create featureprofiles-ci --repository-format=docker --location=us-west1
```

To build the release locally and upload using Docker:

```
docker build . -t featureprofiles-ci-trigger -f tools/ci-trigger/Dockerfile
docker tag featureprofiles-ci-trigger:latest us-west1-docker.pkg.dev/disco-idea-817/featureprofiles-ci/featureprofiles-ci-trigger:latest
docker push us-west1-docker.pkg.dev/disco-idea-817/featureprofiles-ci/featureprofiles-ci-trigger:latest
```

To deploy the container into the project:

```
gcloud run deploy featureprofiles-ci-trigger --cpu 2000m --memory 2Gi --region us-west1 --image us-west1-docker.pkg.dev/disco-idea-817/featureprofiles-ci/featureprofiles-ci-trigger:latest
```

Allow for background CPU and a minimum instance count for pubsub pull to continue processing.

```
gcloud run services update featureprofiles-ci-trigger --region us-west1 --no-cpu-throttling --min-instances 1
```

#### Cloud Secrets

Create the secrets and set them.

```
gcloud secrets create featureprofiles-ci-github-webhook --replication-policy="automatic"
echo -n "secret data" | gcloud secrets versions add featureprofiles-ci-github-webhook --data-file=-
gcloud secrets create featureprofiles-ci-api-secret --replication-policy="automatic"
echo -n "secret data" | gcloud secrets versions add featureprofiles-ci-api-secret --data-file=-

gcloud secrets add-iam-policy-binding featureprofiles-ci-github-webhook --member="serviceAccount:serviceAccountName@developer.gserviceaccount.com" --role="roles/secretmanager.secretAccessor"
gcloud secrets add-iam-policy-binding featureprofiles-ci-api-secret --member="serviceAccount:serviceAccountName@developer.gserviceaccount.com" --role="roles/secretmanager.secretAccessor"
```

Expose `/etc/secrets/github-webhook-secret/github-webhook-secret` and `/etc/secrets/github-api-secret/github-api-secret` to the Cloud Run container.

```
gcloud run deploy featureprofiles-ci-trigger --region us-west1 --image us-west1-docker.pkg.dev/disco-idea-817/featureprofiles-ci/featureprofiles-ci-trigger:latest --update-secrets=/etc/secrets/github-webhook-secret/github-webhook-secret=featureprofiles-ci-github-webhook:latest
gcloud run deploy featureprofiles-ci-trigger --region us-west1 --image us-west1-docker.pkg.dev/disco-idea-817/featureprofiles-ci/featureprofiles-ci-trigger:latest --update-secrets=/etc/secrets/github-api-secret/github-api-secret=featureprofiles-ci-api-secret:latest
```

## Development

A local copy of the cloud function can be launched with the following commands:

```
export GITHUB_WEBHOOK_SECRET=shared_secret
export GITHUB_API_SECRET=api_secret

umask 0022
go run github.com/openconfig/featureprofiles/tools/ci-trigger -alsologtostderr -badge_pubsub=false
```

Alternatively, the docker image can be run locally with the following:

```
docker build -t ci-trigger:latest -f tools/ci-trigger/Dockerfile .
docker run -v ~/.config:/root/.config -e GITHUB_WEBHOOK_SECRET -e GITHUB_API_SECRET -p 8080:8080  ci-trigger:latest -alsologtostderr -badge_pubsub=false
```

You may need to customize the config.go files based on your environment.  You will also need to have some form of [Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials) available.
