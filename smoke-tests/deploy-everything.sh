#!/bin/sh

set -ex

BASEDIR="$(dirname "$(realpath "$0")")"
export CONFIGPATH="${BASEDIR}/k0sctl.yaml"

k0sctl apply --debug --config "${CONFIGPATH}"

k0sctl kubeconfig --config "${CONFIGPATH}" >"${BASEDIR}/kubeconfig"
chmod 600 ./kubeconfig
export KUBECONFIG="${BASEDIR}/kubeconfig"

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.4/cert-manager.yaml

kubectl apply -f certificates/

kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.26/deploy/local-path-storage.yaml

kubectl apply -f manifests/registry/
kubectl apply -f manifests/dkv/
