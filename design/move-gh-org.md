# Plan for moving the Velero GitHub repo into the VMware GitHub organization 

Status: {Draft}

Currently, the Velero repository sits under the Heptio GitHub organization. With the acquisition of Heptio by VMware, it is due time that this repo moves to one of the VMware GitHub organizations. This document outlines a plan to move this repo to the VMware Tanzu (https://github.com/vmware-tanzu) organization.


## Goals

- List all steps necessary to have this repo fully functional under the new org

## Non Goals

- Highlight any step necessary around setting up the new organization and its members

## Steps

- Use GH UI to transfer the repository to the VMW org: https://help.github.com/en/articles/transferring-a-repository
- [ ] Find/replace in all Go, script, yaml, documentation, and website files: `github.com/heptio/velero -> github.com/vmware-tanzu/velero`
- [ ] Change deployment and grpc-push scripts with the new location path.

Each individual developer should point their origin to the new location: `git remote set-url origin git@github.com:vmware-tanzu/velero.git`

## Notes

Might want to exclude updating documentation prior to v0.10.
GH documentation does not specify if branches on the server are also moved.
All links to the previous repository location are automatically redirected to the new location.

## Alternatives Considered

Alternatives such as moving Velero to its own organization, or even not moving at all, were considered. Collectively, however, the open source leadership decided it would be best to move it so it lives alongside other cloud native related repositories.

## Security Considerations

- Ensure that only the Velero core team has maintainer privileges.
