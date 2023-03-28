---
authors: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD X - GCP auto-discovery

## Required Approvers

## What

Teleport discovery services will be able to automatically discover and enroll GCP virtual machine
instances.

link to ec2 and azure join

## Why

## Details

### Discovery

Support for auto-discovering GCP VMs will be added to the Discovery Service.

```yaml
discovery_service:
  enabled: 'yes'
  gcp:
    - types: ['vm']
      project_ids: []
      locations: []
      tags:
        'teleport': 'yes'
      install:
        join_params:
          token_name: 'gcp-discovery-token' # default value
        nodename:
        script_name: 'default-installer' # default value
```

New GCP nodes will be discovered periodically on a 5 minute timer, as new
nodes are found they will be added to the teleport cluster.

In order to avoid attempting to reinstall Teleport on top of an instance where it is
already present, the generated Teleport config will match against the node name using
the project ID, zone, and instance ID by default. This can be overridden
by specifying a node name in the join params.

```json
{
  "kind": "node",
  "version": "v2",
  "metadata": {
    "name": "<project-id>-<zone>-<instance-id>",
    "labels": {
      "env": "example",
      "teleport.internal/discovered-node": "yes",
      "teleport.internal/discovered-by": "<discovered-node-uuid>",
      "teleport.internal/origin": "cloud",
      "teleport.internal/zone": "<zone>",
      "teleport.internal/projectId": "88888888-8888-8888-8888-888888888888",
      "teleport.internal/instanceId": ""
    }
  },
  "spec": {
    "public_addr": "...",
    "hostname": "gcpxyz"
  }
}
```

### Agent installation

In order to install the Teleport agent on GCP virtual machines, Teleport will serve an
install script at `/webapi/scripts/{installer-resource-name}`. Installer scripts will
be editable as a resource.

Example resource script:

```yaml
kind: installer
metadata:
  name: 'installer' # default value
spec:
  # shell script that will be downloaded and run by the virtual machine
  script: |
    #!/bin/sh
    curl https://.../teleport-pubkey.asc ...
    echo "deb [signed-by=... stable main" | tee ... > /dev/null
    apt-get update
    apt-get install teleport
    teleport node configure --auth-agent=... --join-method=gcp --token-name=azure-token
  # Any resource in Teleport can automatically expire.
  expires: 0001-01-01T00:00:00Z
```

Unless overridden by a user, a default teleport installer command will be
generated that is appropriate for the current running version and operating
system initially supporting DEB and RPM based distros that Teleport already
provides packages for.

To run commands on a VM, the Discovery Service will create a short-lived
ssh key pair and add the public key to the VM via its metadata. Then it will
run the installer on the VM over SSH.

> Note: GCP VMs using [OS Login](https://cloud.google.com/compute/docs/oslogin) do not support SSH keys in instance metadata.

The Discovery Service's service account will require the following permissions:

- `compute.instances.setMetadata`
- `compute.instances.get`
- `compute.instances.list`

### GCP join method

In order to register GCP virtual machines, a new `gcp` join method will be
created. The `gcp` join method will be
an oidc-based join method like `github`, `kubernetes`, etc. The token will be fetched from the VM's
instance metadata, with the audience claim set to the name of the Teleport
cluster. The rest of the registration process will be identical to that of the
[other oidc join methods](link to oidc rfd).

The joining VM will need a [service account](https://cloud.google.com/compute/docs/access/create-enable-service-accounts-for-instances)
assigned to it to be able to generate id tokens. No permissions on the account
are needed.

#### Teleport Configuration

The existing provision token type can be extended to support GCP
authentication, using new GCP-specific fields in the token rules section.

```yaml
kind: token
version: v2
metadata:
  name: example_gcp_token
spec:
  roles: [Node, Kube, Db]
  gcp:
    allow:
      # IDs of projects from which nodes can join. At least one required.
      - project_ids: ['p1', 'p2']
        # Location from which nodes can join. If empty or omitted, nodes from
        # any location are allowed.
        locations: ['l1', 'l2']
```

teleport.yaml on the nodes should be configured so that they will use the GCP
join token:

```yaml
teleport:
  join_params:
    token_name: 'example_gcp_token'
    method: gcp
```

### teleport.yaml generation

The `teleport node configure` subcommand will be used to generate a
new /etc/teleport.yaml file:

```sh
teleport node configure
    --auth-server=auth-server.example.com [auth server that is being connected to]
    --token="$1" # name of the join token, passed via parameter from run-command
    --labels="teleport.internal/projectId=${PROJECT_ID},\
    teleport.internal/zone=${ZONE},\
    teleport.internal/discovered-node=yes,\
    teleport.internal/discovered-by=$2,\
    teleport.internal/origin=cloud" # sourced from instance metadata
```

This will generate a file with the following contents:

```yaml
teleport:
  nodename: '<project-id>-<zone>-<instance-id>'
  auth_servers:
    - 'auth-server.example.com:3025'
  join_params:
    token_name: token
  # ...
ssh_service:
  enabled: 'yes'
  labels:
    teleport.internal/projectId: '<project-id>'
    teleport.internal/zone: '<zone>'
```

## UX

### User has 1 account to discover servers on

#### Teleport config

Discovery server:

```yaml
teleport: ...
auth_service:
  enabled: 'yes'
discovery_service:
  enabled: 'yes'
  gcp:
    - types: ['vm']
      project_ids: ['<project_id>']
      locations: ['westcentralus']
      tags:
        'teleport': 'yes'
      install:
        # Use default values
```

custom role for discovery service

```yaml
title: teleport_vm_discovery
description: Role for discovering VMs and adding them to a Teleport cluster.
stage: ALPHA
includedPermissions:
  - compute.instances.setMetadata
  - compute.instances.get
  - compute.instances.list
```

```text
gcloud iam roles create ROLE_ID --project=PROJECT_ID \
    --file=YAML_FILE_PATH
```

## Security considerations

- no ssm to separate permissions for creating/updating commands
- no "run command" operation (and no dedicated permission)

## Appendix I - Example ID token payload
