#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

source "${CODEGEN_PKG}/kube_codegen.sh"
THIS_PKG="github.com/Sumon009838/CRD"
echo ${SCRIPT_ROOT}
kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/copyright.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

kube::codegen::gen_client \
    --with-watch \
    --output-dir "${SCRIPT_ROOT}/pkg/generated" \
    --output-pkg "${THIS_PKG}/pkg/generated" \
    --boilerplate "${SCRIPT_ROOT}/hack/copyright.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

#
echo "here"

OUTPUT_DIR="./manifests"
ABSOLUTE_PATH=$(realpath "$OUTPUT_DIR")

# Print the current working directory
echo "Current working directory is: $(pwd)"

# Print the resolved absolute path
echo "Resolved absolute path is: $ABSOLUTE_PATH"

# Check if the directory exists
if [[ -d "$ABSOLUTE_PATH" ]]; then
  echo "Directory exists."
else
  echo "Directory does not exist. Creating it now..."
  mkdir -p "$ABSOLUTE_PATH"
fi

# Now run your controller-gen command
controller-gen rbac:roleName=my-crd-controller crd \
  paths=github.com/Sumon009838/CRD/pkg/apis/reader.com/v1 \
  output:crd:dir="$ABSOLUTE_PATH" output:stdout