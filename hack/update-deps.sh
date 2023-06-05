#!/usr/bin/env bash

# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

source $(dirname "$0")/../vendor/knative.dev/hack/library.sh

CONTOUR_VERSION="v1.25.0" # This is for controlling which version of contour we want to use.

CLUSTER_ROLE_NAME=knative-contour

FLOATING_DEPS=(
  "github.com/projectcontour/contour@${CONTOUR_VERSION}"
)

go_update_deps "$@"

function run_ytt() {
  go_run github.com/vmware-tanzu/carvel-ytt/cmd/ytt@v0.45.1 "$@"
}

function contour_yaml() {
  # Used to be: KO_DOCKER_REPO=ko.local ko resolve -f ./vendor/github.com/projectcontour/contour/examples/contour/
  curl "https://raw.githubusercontent.com/projectcontour/contour/${CONTOUR_VERSION}/examples/render/contour.yaml"
}

rm -rf config/contour/*

contour_yaml | \
  run_ytt --ignore-unknown-comments \
    --data-value namespace=contour-internal \
    --data-value clusterrole.name=$CLUSTER_ROLE_NAME \
    -f hack/overlays \
    -f - >> config/contour/internal.yaml

contour_yaml | \
  run_ytt --ignore-unknown-comments \
    --data-value namespace=contour-external \
    --data-value clusterrole.name=$CLUSTER_ROLE_NAME \
    -f hack/overlays \
    -f - >> config/contour/external.yaml

