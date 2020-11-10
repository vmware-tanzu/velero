# Plan for moving the Velero GitHub repo into the VMware GitHub organization

Currently, the Velero repository sits under the Heptio GitHub organization. With the acquisition of Heptio by VMware, it is due time that this repo moves to one of the VMware GitHub organizations. This document outlines a plan to move this repo to the VMware Tanzu (https://github.com/vmware-tanzu) organization.

## Goals

- List all steps necessary to have this repo fully functional under the new org

## Non Goals

- Highlight any step necessary around setting up the new organization and its members

## Action items

### Todo list

#### Pre move

- [ ] PR: Blog post communicating the move. https://github.com/heptio/velero/issues/1841. Who: TBD.
- [ ] PR: Find/replace in all Go, script, yaml, documentation, and website files: `github.com/heptio/velero -> github.com/vmware-tanzu/velero`. Who: a Velero developer; TBD
- [ ] PR: Update website with the correct GH links. Who: a Velero developer; TBD
- [ ] PR: Change deployment and grpc-push scripts with the new location path. Who: a Velero developer; TBD
- [ ] Delete branches not to be carried over (https://github.com/heptio/velero/branches/all). Who: Any of the current repo owners; TBD

#### Move

- [ ] Use GH UI to transfer the repository to the VMW org; must be accepted within a day. Who: new org owner; TBD
- [ ] Make owners of this repo owners of repo in the new org. Who: new org owner; TBD
- [ ] Update Travis CI. Who: Any of the new repo owners; TBD
- [ ] Add DCO for signoff check (https://probot.github.io/apps/dco/). Who: Any of the new repo owners; TBD


#### Post move

- [ ] Each individual developer should point their origin to the new location: `git remote set-url origin git@github.com:vmware-tanzu/velero.git`
- [ ] Transfer ZenHub. Who: Any of the new repo owners; TBD
- [ ] Update Netlify deploy settings. Any of the new repo owners; TBD
- [ ] GH app: Netlify integration. Who: Any of the new repo owners; TBD
- [ ] GH app: Slack integration. Who: Any of the new repo owners; TBD
- [ ] Add webhook: travis CI. Who: Any of the new repo owners; TBD
- [ ] Add webhook: zenhub. Who: Any of the new repo owners; TBD
- [ ] Move all 3 native provider plugins into their own individual repo. https://github.com/heptio/velero/issues/1537. Who: @carlisia.
- [ ] Merge PRs from the "pre move" section
- [ ] Create a team for the Velero core members (https://github.com/orgs/vmware-tanzu/teams/). Who: Any of the new repo owners; TBD

### Notes/How-Tos

#### Transferring the GH repository

All action items needed for the repo transfer are listed in the Todo list above. For details about what gets moved and other info, this is the GH documentation: https://help.github.com/en/articles/transferring-a-repository

[Pending] We will find out this week who will be the organization owner(s) who will accept this transfer in the new GH org. This organization owner will make all current owners in this repo owners in the new org Velero repo.

#### Updating Travis CI

Someone with owner permission on the new repository needs to go to their Travis CI account and authorize Travis CI on the repo. Here are instructions: https://docs.travis-ci.com/user/tutorial/.

After this, webhook notifications can be added following these instructions: https://docs.travis-ci.com/user/notifications/#configuring-webhook-notifications.

#### Transferring ZenHub

Pre-requisite: A new Zenhub account must exist for a vmware or vmware-tanzu organization.

This page contains a pre-migration checklist for ensuring a repo migration goes well with Zenhub: https://help.zenhub.com/support/solutions/articles/43000010366-moving-a-repo-cross-organization-or-to-a-new-organization. After this, webhooks can be added by following these instructions: https://github.com/ZenHubIO/API#webhooks.

#### Updating Netlify

The settings for Netflify should remain the same, except that it now needs to be installed in the new repo. The instructions on how to install Netlify on the new repo are here: https://www.netlify.com/docs/github-permissions/.

#### Communication strategy

[Pending] We will find out this week how this move will be communicated to the community. In particular, the Velero repository move might be tied to the move of our provider plugins into their own repos, also in the new org: https://github.com/heptio/velero/issues/1814.

#### TBD

Many items on the todo list must be done by a repository member with owner permission. This doesn't all need to be done by the same person obviously, but we should specify if @skriss wants to split these tasks with any other owner(s).

#### Other notes

Might want to exclude updating documentation prior to v1.0.0.
GH documentation does not specify if branches on the server are also moved.
All links to the original repository location are automatically redirected to the new location.

## Alternatives Considered

Alternatives such as moving Velero to its own organization, or even not moving at all, were considered. Collectively, however, the open source leadership decided it would be best to move it so it lives alongside other VMware supported cloud native related repositories.

## Security Considerations

- Ensure that only the Velero core team has maintainer/owner privileges.
