# Plan for moving the Velero GitHub repo into the VMware GitHub organization

Currently, the Velero repository sits under the Heptio GitHub organization. With the acquisition of Heptio by VMware, it is due time that this repo moves to one of the VMware GitHub organizations. This document outlines a plan to move this repo to the VMware Tanzu (https://github.com/vmware-tanzu) organization.

## Goals

- List all steps necessary to have this repo fully functional under the new org

## Non Goals

- Highlight any step necessary around setting up the new organization and its members

## Action items

### Todo list

- [ ] Write blog post communicating the move. https://github.com/heptio/velero/issues/1841. Who: TBD.
- [ ] Move all provider plugins into their own repo. https://github.com/heptio/velero/issues/1537. Who: @carlisia.
- [ ] Use GH UI to transfer the repository to the VMW org; must be accepted within a day. Who: new org owner; TBD
- [ ] Make owners of this repo owners of repo in the new org. Who: new org owner; TBD
- [ ] Find/replace in all Go, script, yaml, documentation, and website files: `github.com/heptio/velero -> github.com/vmware-tanzu/velero`. Who: a Velero developer; TBD
- [ ] Change deployment and grpc-push scripts with the new location path. Who: a Velero developer; TBD
- [ ] Update Travis CI. Who: Any of the new repo owners; TBD
- [ ] Transfer ZenHub. Who: Any of the new repo owners; TBD
- [ ] Update Netlify deploy settings. Any of the new repo owners; TBD
- [ ] GH app: Netlify integration. Who: Any of the new repo owners; TBD
- [ ] GH app: Slack integration. Who: Any of the new repo owners; TBD
- [ ] Add webhook: signoff checker. Who: Any of the new repo owners; TBD
- [ ] Add webhook: travis CI. Who: Any of the new repo owners; TBD
- [ ] Add webhook: zenhub. Who: Any of the new repo owners; TBD
- [ ] Each individual developer should point their origin to the new location: `git remote set-url origin git@github.com:vmware-tanzu/velero.git`. 

### Notes/How-Tos

#### Transfering the GH repository

All action items needed for the repo transfer are listed in the Todo list above. For deailts about what gets moved and other info, this is the GH documentation: https://help.github.com/en/articles/transferring-a-repository

[Pending] We will find out this week who will be the organization owner(s) who will accept this transfer in the new GH org. This organization owner will make all current owners in this repo owners in the new org Velero repo.

#### Updating Travis CI

Someone with owner permission on the new repository needs to go to their Travis CI account and authorize Travis CI on the repo. Here are instructions: https://docs.travis-ci.com/user/tutorial/.

After this, a webhook notifications can be added following these instructions: https://docs.travis-ci.com/user/notifications/#configuring-webhook-notifications.

#### Signoff checker

TBD: waiting for instructions from the OSPO leadership.

#### Transfering ZenHub

Pre-requisite: A new Zenhub account must exist for a vmware or vmware-tanzu organization.

This page contains a pre-miration checklist for ensuring a repo migration goes well with Zenhub: https://help.zenhub.com/support/solutions/articles/43000010366-moving-a-repo-cross-organization-or-to-a-new-organization. After this, a webhook will need to be added for desired notifications; instructions here: https://docs.travis-ci.com/user/tutorial/.

After this, webhooks can be added by following these instructions: https://github.com/ZenHubIO/API#webhooks.

#### Updating Netlify

The settings for Netflify should remain the same, except that it now needs to be installed in the new repo. The instructions on how to install Netlify on the new repo are here: https://www.netlify.com/docs/github-permissions/.

#### Communication strategy

[Pending] We will find out this week how this move will be communicated to the community. In particular, the Velero repository move might be tied to the move of our provider plugins into their own repos, also in the new org: https://github.com/heptio/velero/issues/1814.

#### TBD

Many items on the todo list must be done by a repository member with owner permission. This don't all need to be done by the same person obviously, but we should specify if @skriss wants to split these tasks with any other owner(s).

#### Other notes

Might want to exclude updating documentation prior to v0.10.
GH documentation does not specify if branches on the server are also moved.
All links to the original repository location are automatically redirected to the new location.

## Alternatives Considered

Alternatives such as moving Velero to its own organization, or even not moving at all, were considered. Collectively, however, the open source leadership decided it would be best to move it so it lives alongside other VMware supported cloud native related repositories.

## Security Considerations

- Ensure that only the Velero core team has maintainer/owner privileges.
