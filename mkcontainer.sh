#!/bin/sh

tag=$(git rev-parse --short HEAD)
tag=${tag}_4
containerid=$(buildah from debian:buster)
buildah copy ${containerid} bin/phs /usr/local/bin/phs
buildah config --cmd /usr/local/bin/phs ${containerid}
buildah commit ${containerid} registry.home.lausch.at/k8s-experiments/webapp1/phs:$tag
buildah commit ${containerid} registry.home.lausch.at/k8s-experiments/webapp1/phs:latest
buildah images
podman push registry.home.lausch.at/k8s-experiments/webapp1/phs:$tag
podman push registry.home.lausch.at/k8s-experiments/webapp1/phs:latest
