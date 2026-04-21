# Multi-stage build for OVN Kubernetes with ECMP duplicate routes fix (OCPBUGS-70272)
# Stage 1: Build the ovnkube binary with the PolicyEqualPredicate fix
FROM golang:1.25 AS builder

# Install required build dependencies
RUN apt-get update && apt-get install -y gcc libc6-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /go/src/github.com/ovn-org/ovn-kubernetes
COPY . .
RUN cd go-controller && CGO_ENABLED=0 make

# Stage 2: Use the current cluster's OVN image as base and replace binary
FROM quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:53ecbed423371b09c2867ebbc1ad10dbc259eb69a1dadb0162a491548a503d8a

USER root

# Replace the ovnkube binary with our fixed version
COPY --from=builder /go/src/github.com/ovn-org/ovn-kubernetes/go-controller/_output/go/bin/ovnkube /usr/bin/ovnkube

# Ensure binary is executable
RUN chmod +x /usr/bin/ovnkube

WORKDIR /root
ENTRYPOINT /root/ovnkube.sh
