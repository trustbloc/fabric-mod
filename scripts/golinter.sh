#!/bin/bash

# Copyright Greg Haskins All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -e

# shellcheck source=/dev/null
source "$(cd "$(dirname "$0")" && pwd)/functions.sh"

fabric_dir="$(cd "$(dirname "$0")/.." && pwd)"
source_dirs=()
while IFS=$'\n' read -r source_dir; do
    source_dirs+=("$source_dir")
done < <(go list -f '{{.Dir}}' ./... | sed s,"${fabric_dir}".,,g | cut -f 1 -d / | sort -u)

echo "Checking with gofmt"
OUTPUT="$(gofmt -l -s "${source_dirs[@]}")"
OUTPUT="$(filterExcludedAndGeneratedFiles "$OUTPUT")"
if [ -n "$OUTPUT" ]; then
    echo "The following files contain gofmt errors"
    echo "$OUTPUT"
    echo "The gofmt command 'gofmt -l -s -w' must be run for these files"
    exit 1
fi

#TODO goimports f60e5f99f0816fc2d9ecb338008ea420248d2943 doesn't work with go modules
#we need to upgrade goimports but then we have to goimports 40 files
#which cause merge conflict when we upgrade
echo "Checking with goimports"
#OUTPUT="$(goimports -l ${source_dirs} | grep -Ev '(^|/)testdata/' || true)"
#OUTPUT="$(goimports -l "${source_dirs[@]}")"
#OUTPUT="$(filterExcludedAndGeneratedFiles "$OUTPUT")"
if [ -n "$OUTPUT" ]; then
    echo "The following files contain goimports errors"
    echo "$OUTPUT"
    echo "The goimports command 'goimports -l -w' must be run for these files"
    exit 1
fi

# Now that context is part of the standard library, we should use it
# consistently. The only place where the legacy golang.org version should be
# referenced is in the generated protos.
echo "Checking for golang.org/x/net/context"
context_whitelist=(
    "^github.com/hyperledger/fabric/core/comm/testpb:"
    "^github.com/hyperledger/fabric/orderer/common/broadcast/mock:"
    "^github.com/hyperledger/fabric/common/grpclogging/fakes:"
    "^github.com/hyperledger/fabric/common/grpclogging/testpb:"
    "^github.com/hyperledger/fabric/common/grpcmetrics/fakes:"
    "^github.com/hyperledger/fabric/common/grpcmetrics/testpb:"
)
# shellcheck disable=SC2016
TEMPLATE='{{with $d := .}}{{range $d.Imports}}{{ printf "%s:%s " $d.ImportPath . }}{{end}}{{end}}'
OUTPUT="$(go list -f "$TEMPLATE" ./... | grep -Ev "$(IFS='|' ; echo "${context_whitelist[*]}")" | grep 'golang.org/x/net/context' | cut -f1 -d:)"
if [ -n "$OUTPUT" ]; then
    echo "The following packages import golang.org/x/net/context instead of context"
    echo "$OUTPUT"
    exit 1
fi

echo "Checking with go vet"
PRINTFUNCS="Print,Printf,Info,Infof,Warning,Warningf,Error,Errorf,Critical,Criticalf,Sprint,Sprintf,Log,Logf,Panic,Panicf,Fatal,Fatalf,Notice,Noticef,Wrap,Wrapf,WithMessage"
OUTPUT="$(go vet -all -printfuncs "$PRINTFUNCS" ./...)"
if [ -n "$OUTPUT" ]; then
    echo "The following files contain go vet errors"
    echo "$OUTPUT"
    exit 1
fi
