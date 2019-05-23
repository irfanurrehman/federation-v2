#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/demo_util.sh"
source "$(dirname "${BASH_SOURCE}")/../util.sh"

OVERWRITE_KUBECONFIG=y KIND_TAG="v1.13.4" ./scripts/create-clusters.sh

./scripts/deploy-federation.sh 172.17.0.1:5000/kubefed:e2e cluster2