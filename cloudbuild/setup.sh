#!/bin/bash

set -xe

GOLANG_URL="https://dl.google.com/go/go1.17.6.linux-amd64.tar.gz"

# Install Go
curl -o go.tar.gz ${GOLANG_URL}
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
export PATH=${PATH}:/usr/local/go/bin
export PATH=${PATH}:$(go env GOPATH)/bin

# Install Docker
sudo apt-get update
sudo apt-get install apt-transport-https ca-certificates curl gnupg lsb-release -y
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get install docker-ce docker-ce-cli containerd.io build-essential -y
sudo chown "${USER}" /var/run/docker.sock
gcloud auth configure-docker gcr.io -q

# Install kind, kubectl
go install sigs.k8s.io/kind@latest
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install KNE
git clone https://github.com/google/kne.git
pushd kne/kne_cli
go install -v
popd
kne_cli deploy kne/deploy/kne/kind.yaml
