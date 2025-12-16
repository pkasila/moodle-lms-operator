#!/usr/bin/env bash

# A script to automate testing the Moodle LMS Operator in a Kind cluster.
# This version includes setup for a local storage provisioner and a PostgreSQL database.

set -euo pipefail

# --- Configuration ---
KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-"moodle-lms-operator-test"}
KUBECTL="kubectl"
KIND="kind"
IMG="controller:test"
LOCAL_PATH_PROVISIONER_URL="https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.32/deploy/local-path-storage.yaml"
DB_NAMESPACE="db"
DB_SECRET_NAME="postgres-admin-creds"
DB_ADMIN_USER="postgres"
DB_ADMIN_PASSWORD="supersecretpassword"

MOODLE_DB_NAME="moodle"
MOODLE_DB_USER="moodle"
MOODLE_DB_PASSWORD="moodlepassword"

# --- Colors for logging ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO] ${1}${NC}"
}

warn() {
    echo -e "${YELLOW}[WARN] ${1}${NC}"
}

error() {
    echo -e "${RED}[ERROR] ${1}${NC}"
    exit 1
}

# --- Functions ---

# Creates a Kind cluster if it doesn't already exist.
setup_cluster() {
    info "Checking for Kind cluster '${KIND_CLUSTER_NAME}'..."
    if ! "${KIND}" get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
        info "Creating Kind cluster '${KIND_CLUSTER_NAME}'..."
        "${KIND}" create cluster --name "${KIND_CLUSTER_NAME}"
    else
        info "Kind cluster '${KIND_CLUSTER_NAME}' already exists."
    fi
    "${KUBECTL}" cluster-info --context "kind-${KIND_CLUSTER_NAME}"
}

# Deploys the local-path-provisioner for dynamic storage.
deploy_storage_provisioner() {
    info "Deploying local-path-provisioner..."
    "${KUBECTL}" apply -f "${LOCAL_PATH_PROVISIONER_URL}"
    info "Waiting for local-path-provisioner to be ready..."
    "${KUBECTL}" wait deployment/local-path-provisioner \
        --for=condition=Available \
        --namespace=local-path-storage \
        --timeout=120s
}

# Deploys nginx-ingress controller for ingress support.
deploy_nginx_ingress() {
    info "Deploying nginx-ingress controller..."
    "${KUBECTL}" apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
    
    info "Waiting for ingress-nginx admission webhook jobs to complete..."
    sleep 10
    "${KUBECTL}" wait --namespace ingress-nginx \
        --for=condition=complete \
        --timeout=90s \
        job/ingress-nginx-admission-create 2>/dev/null || warn "Admission create job did not complete in time"
    
    "${KUBECTL}" wait --namespace ingress-nginx \
        --for=condition=complete \
        --timeout=30s \
        job/ingress-nginx-admission-patch 2>/dev/null || warn "Admission patch job did not complete in time"
    
    info "Waiting for nginx-ingress controller to be ready..."
    "${KUBECTL}" wait --namespace ingress-nginx \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=240s
}

# Deploys a PostgreSQL database for the operator to use.
deploy_database() {
    info "Deploying PostgreSQL database..."
    # Create and label the namespace for the database
    cat <<EOF | "${KUBECTL}" apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${DB_NAMESPACE}
  labels:
    moodle.bsu.by/db: "true"
EOF

    # Create a secret for the postgres admin password
    "${KUBECTL}" create secret generic "${DB_SECRET_NAME}" \
        --namespace="${DB_NAMESPACE}" \
        --from-literal=POSTGRES_USER="${DB_ADMIN_USER}" \
        --from-literal=POSTGRES_PASSWORD="${DB_ADMIN_PASSWORD}" \
        --dry-run=client -o yaml | "${KUBECTL}" apply -f -

    # Create the PostgreSQL Deployment and Service
    cat <<EOF | "${KUBECTL}" apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  namespace: ${DB_NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15-alpine
        env:
        - name: POSTGRES_DB
          value: "${MOODLE_DB_NAME}"
        - name: POSTGRES_USER
          value: "${MOODLE_DB_USER}"
        - name: POSTGRES_PASSWORD
          value: "${MOODLE_DB_PASSWORD}"
        ports:
        - containerPort: 5432
        readinessProbe:
          exec:
            command:
            - pg_isready
            - -U
            - ${MOODLE_DB_USER}
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: ${DB_NAMESPACE}
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
EOF

    info "Waiting for PostgreSQL to be ready..."
    "${KUBECTL}" wait deployment/postgres \
        --for=condition=Available \
        --namespace="${DB_NAMESPACE}" \
        --timeout=180s
}

# Builds the operator and custom Moodle images and loads them into the Kind cluster.
build_and_load_image() {
    info "Building operator image '${IMG}'..."
    make docker-build IMG=${IMG}
    info "Loading operator image into Kind cluster..."
    "${KIND}" load docker-image "${IMG}" --name "${KIND_CLUSTER_NAME}"

    info "Building custom Moodle image 'moodle-lms-operator/moodle:custom'..."
    docker build -t "moodle-lms-operator/moodle:custom" ./moodle-image
    info "Loading custom Moodle image into Kind cluster..."
    "${KIND}" load docker-image "moodle-lms-operator/moodle:custom" --name "${KIND_CLUSTER_NAME}"
}

# Applies a sample MoodleTenant CR and verifies the created resources.
test_deployment() {
    info "Applying sample MoodleTenant resource..."
    
    # Apply a modified sample that points to our in-cluster DB
    cat <<EOF | "${KUBECTL}" apply -f -
apiVersion: moodle.bsu.by/v1alpha1
kind: MoodleTenant
metadata:
  name: moodle-sample
spec:
  hostname: moodle-sample.example.com
  image: "moodle-lms-operator/moodle:custom" # Use our custom-built image
  databaseRef:
    host: "postgres.db.svc.cluster.local"
    adminSecret: "moodle-db-admin"
    name: "${MOODLE_DB_NAME}"
    user: "${MOODLE_DB_USER}"
    password: "${MOODLE_DB_PASSWORD}"
  storage:
    size: 1Gi
    storageClass: "local-path" # Use the provisioner we installed
  hpa:
    enabled: false # Disable HPA for this simple test
EOF

    # Give the operator some time to reconcile
    info "Waiting for 60 seconds for reconciliation..."
    sleep 60

    info "--- Verification ---"
    local tenant_name="moodle-sample"
    local tenant_namespace="tenant-${tenant_name}"

    info "Checking for namespace '${tenant_namespace}'..."
    if ! "${KUBECTL}" get namespace "${tenant_namespace}" > /dev/null; then
        error "Namespace '${tenant_namespace}' was not created."
    fi
    info "✅ Namespace found."

    info "Checking for PersistentVolumeClaim in '${tenant_namespace}'..."
    local pvc_status=$("${KUBECTL}" get pvc -n "${tenant_namespace}" "${tenant_name}-data" -o jsonpath='{.status.phase}')
    if [[ "${pvc_status}" != "Bound" ]]; then
        error "PVC was not bound. Status is: ${pvc_status}"
    fi
    info "✅ PVC is Bound."

    info "Checking for Deployment in '${tenant_namespace}'..."
    if ! "${KUBECTL}" get deployment -n "${tenant_namespace}" "${tenant_name}-deployment" > /dev/null; then
        error "Deployment for '${tenant_name}' was not created."
    fi
    info "✅ Deployment found."

    info "Waiting for Moodle Deployment to be ready..."
    # The Moodle image can be slow to start, so we give it a long timeout
    if ! "${KUBECTL}" wait deployment/"${tenant_name}-deployment" \
        --for=condition=Available \
        --namespace="${tenant_namespace}" \
        --timeout=300s; then
        
        error "Moodle deployment did not become ready. Checking pod logs..."
        "${KUBECTL}" logs -n "${tenant_namespace}" "deployment/${tenant_name}-deployment" --all-containers --tail=100
        exit 1
    fi
    info "✅ Moodle Deployment is ready."

    info "Checking for Service in '${tenant_namespace}'..."
    if ! "${KUBECTL}" get service -n "${tenant_namespace}" "${tenant_name}-service" > /dev/null; then
        error "Service for '${tenant_name}' was not created."
    fi
    info "✅ Service found."

    info "Checking for Ingress in '${tenant_namespace}'..."
    if ! "${KUBECTL}" get ingress -n "${tenant_namespace}" "${tenant_name}-ingress" > /dev/null; then
        error "Ingress for '${tenant_name}' was not created."
    fi
    info "✅ Ingress found."

    info "🎉 Full E2E verification successful!"
}

# Cleans up the created resources.
cleanup() {
  exit 1
    warn "Cleaning up test resources..."
    
    # Delete the MoodleTenant and secret
    "${KUBECTL}" delete moodletenant moodle-sample --ignore-not-found=true
    
    # Wait for the operator to delete the namespace
    info "Waiting for operator to clean up tenant namespace..."
    sleep 15
    
    # Undeploy operator
    make undeploy IMG=${IMG} 2>/dev/null || true
    make uninstall 2>/dev/null || true

    # Clean up database
    warn "Removing database..."
    "${KUBECTL}" delete namespace "${DB_NAMESPACE}" --ignore-not-found=true
}

# Deploys the operator to the cluster.
deploy_operator() {
    info "Installing Custom Resource Definitions (CRDs)..."
    make install
    info "Deploying the controller manager..."
    make deploy IMG=${IMG}
    info "Waiting for operator deployment to be ready..."
    "${KUBECTL}" wait deployment/moodle-lms-operator-controller-manager \
        --for=condition=Available \
        --namespace=moodle-lms-operator-system \
        --timeout=120s
}

# Deletes the Kind cluster.
destroy_cluster() {
    warn "Destroying Kind cluster '${KIND_CLUSTER_NAME}'..."
    "${KIND}" delete cluster --name "${KIND_CLUSTER_NAME}"
}

# --- Main Logic ---
main() {
    trap 'cleanup' EXIT
    
    if [[ "${1:-}" == "cleanup" ]]; then
        cleanup
        exit 0
    fi
    
    if [[ "${1:-}" == "destroy" ]]; then
        destroy_cluster
        exit 0
    fi

    setup_cluster
    deploy_storage_provisioner
    deploy_nginx_ingress
    deploy_database
    build_and_load_image
    deploy_operator
    test_deployment

    info "Test script finished. To clean up resources, run:"
    info "  ./hack/test-in-kind.sh cleanup"
    info "To destroy the cluster, run:"
    info "  ./hack/test-in-kind.sh destroy"
}

main "$@"
