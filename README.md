# k8s-timed-rolebinding

A Kubernetes operator that enables time-bound RBAC permissions through custom resources that automatically create and delete RoleBindings and ClusterRoleBindings based on specified time windows.

## Features

- **TimedRoleBinding**: Namespace-scoped resource for managing time-bound RoleBindings
- **TimedClusterRoleBinding**: Cluster-scoped resource for managing time-bound ClusterRoleBindings
- Automatic cleanup of expired bindings
- Post-activation and post-expiration job hooks
- Configurable retention period for expired resources

## Prerequisites

- Kubernetes 1.19+ (required for webhook support and cert-manager integration)
- kubectl 1.19+
- cert-manager v1.14.3+ (for webhook certificates)

## Installation

### Quick Install

- Install [cert-manager](https://cert-manager.io/docs/installation/) (required for webhooks)

- Install the operator

```sh
kubectl apply -f https://raw.githubusercontent.com/hofman-tan/k8s-timed-rolebinding/main/dist/install.yaml
```

### Building from Source

If you want to build and deploy from source:

```sh
# Clone the repository
git clone https://github.com/hofman-tan/k8s-timed-rolebinding.git
cd k8s-timed-rolebinding

# Build and push the operator image
make docker-build docker-push IMG=<your-registry>/k8s-timed-rolebinding:<tag>

# Generate the installation manifest
make build-installer IMG=<your-registry>/k8s-timed-rolebinding:<tag>

# Apply the manifest against your cluster.
kubectl apply -f dist/install.yaml
```

## Usage

### TimedRoleBinding Example

Basic example that binds `user1` to `role1` for the duration from `startTime` until `endTime`.

```yaml
apiVersion: rbac.hhh.github.io/v1alpha1
kind: TimedRoleBinding
metadata:
  name: timedrolebinding-sample
spec:
  subjects:
    - kind: User
      name: user1
      apiGroup: rbac.authorization.k8s.io
  roleRef:
    kind: Role
    name: role1
    apiGroup: rbac.authorization.k8s.io
  startTime: 2025-01-01T06:00:00Z
  endTime: 2025-01-01T09:00:00Z
```

Example with `postActivate` job hook. Job hooks are job resources automatically created by the controller after its TimedRoleBinding resource becomes activated. The name of the resource can be obtained by referencing the environment variable `TIMED_ROLE_BINDING_NAME` injected into every container of the job.

```yaml
apiVersion: rbac.hhh.github.io/v1alpha1
kind: TimedRoleBinding
metadata:
  name: timedrolebinding-sample
spec:
  subjects:
    - kind: User
      name: user1
      apiGroup: rbac.authorization.k8s.io
  roleRef:
    kind: Role
    name: role1
    apiGroup: rbac.authorization.k8s.io
  startTime: 2025-01-01T06:00:00Z
  endTime: 2025-01-01T09:00:00Z
  keepExpiredFor: 1h
  postActivate:
    jobTemplate:
      spec:
        template:
          spec:
            restartPolicy: Never
            containers:
              - name: post-activate
                image: busybox
                command: ["/bin/sh", "-c"]
                args: ["echo $TIMED_ROLE_BINDING_NAME has been activated"]
  postExpire:
    jobTemplate:
      spec:
        template:
          spec:
            restartPolicy: Never
            containers:
              - name: post-expire
                image: busybox
                command: ["/bin/sh", "-c"]
                args: ["echo $TIMED_ROLE_BINDING_NAME has expired"]
```

#### Field Descriptions

| Field                 | Required | Description                                                                                                       |
| --------------------- | -------- | ----------------------------------------------------------------------------------------------------------------- |
| `spec.subjects`       | Yes      | List of users, groups, or service accounts to grant permissions to. Uses standard Kubernetes RBAC subject format. |
| `spec.roleRef`        | Yes      | Reference to the Role that defines the permissions to grant. Must specify `kind` (Role), `name`, and `apiGroup`.  |
| `spec.startTime`      | Yes      | RFC3339 timestamp when the RoleBinding should be created and permissions granted.                                 |
| `spec.endTime`        | Yes      | RFC3339 timestamp when the RoleBinding should be deleted and permissions revoked. Must be after `startTime`.      |
| `spec.keepExpiredFor` | No       | Duration to keep the TimedRoleBinding resource after expiration (e.g., "1h", "24h", "7d"). Defaults to "24h".     |
| `spec.postActivate`   | No       | Job template to run after the RoleBinding is created. Useful for notifications or setup tasks.                    |
| `spec.postExpire`     | No       | Job template to run after the RoleBinding is deleted. Useful for cleanup or notification tasks.                   |

### TimedClusterRoleBinding Example

Aside from `kind` and the cluster role passed to `.spec.roleRef`, the rest of the specifications are identical to `TimedRoleBinding`. The environment variable `TIMED_CLUSTER_ROLE_BINDING_NAME` stores the name of the TimedClusterRoleBinding that creates the job.

```yaml
apiVersion: rbac.hhh.github.io/v1alpha1
kind: TimedClusterRoleBinding
metadata:
  name: timedclusterrolebinding-sample
spec:
  subjects:
    - kind: User
      name: user1
      apiGroup: rbac.authorization.k8s.io
  roleRef:
    kind: ClusterRole
    name: clusterrole1
    apiGroup: rbac.authorization.k8s.io
  startTime: 2025-01-01T06:00:00Z
  endTime: 2025-01-01T09:00:00Z
  keepExpiredFor: 1h
  postActivate:
    jobTemplate:
      metadata:
        namespace: default # namespace is required to tell the operator where to create the job in
      spec:
        template:
          spec:
            restartPolicy: Never
            containers:
              - name: post-activate
                image: busybox
                command: ["/bin/sh", "-c"]
                args:
                  ["echo $TIMED_CLUSTER_ROLE_BINDING_NAME has been activated"]
```

#### Field Descriptions

| Field                 | Required | Description                                                                                                                    |
| --------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `spec.subjects`       | Yes      | List of users, groups, or service accounts to grant permissions to. Uses standard Kubernetes RBAC subject format.              |
| `spec.roleRef`        | Yes      | Reference to the ClusterRole that defines the permissions to grant. Must specify `kind` (ClusterRole), `name`, and `apiGroup`. |
| `spec.startTime`      | Yes      | RFC3339 timestamp when the ClusterRoleBinding should be created and permissions granted.                                       |
| `spec.endTime`        | Yes      | RFC3339 timestamp when the ClusterRoleBinding should be deleted and permissions revoked. Must be after `startTime`.            |
| `spec.keepExpiredFor` | No       | Duration to keep the TimedClusterRoleBinding resource after expiration (e.g., "1h", "24h", "7d"). Defaults to "24h".           |
| `spec.postActivate`   | No       | Job template to run after the ClusterRoleBinding is created. Must specify `metadata.namespace` for job creation.               |
| `spec.postExpire`     | No       | Job template to run after the ClusterRoleBinding is deleted. Must specify `metadata.namespace` for job creation.               |

## Resource Lifecycle

The lifecycle of TimedRoleBinding and TimedClusterRoleBinding resources follows this sequence:

1. Pending Phase

   - The resource remains in Pending phase until `startTime`.
   - At `startTime`, the operator transitions the resource to Active phase.

2. Active Phase

   - The RoleBinding/ClusterRoleBinding is created and remains active between `startTime` and `endTime`, enforcing RBAC permission.
   - If configured, `postActivate` job executes after binding creation.
   - At `endTime`, the operator transitions the resource to Expired phase.

3. Expired Phase

   - The operator removes the RoleBinding/ClusterRoleBinding.
   - If configured, `postExpire` job executes after binding removal.
   - The resource is retained for `keepExpiredFor` duration before removal.
