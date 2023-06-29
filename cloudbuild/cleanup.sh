#!/bin/bash
sleep 2h
echo 'Execution timeout reached; deleting self'
NAME="$(curl -X GET http://metadata.google.internal/computeMetadata/v1/instance/name -H 'Metadata-Flavor: Google')"
ZONE="$(curl -X GET http://metadata.google.internal/computeMetadata/v1/instance/zone -H 'Metadata-Flavor: Google')"
gcloud --quiet compute instances delete "${NAME}" --zone="${ZONE}"
