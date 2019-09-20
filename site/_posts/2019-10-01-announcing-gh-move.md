---
title: Announcing a new GitHub home for Velero
image: /img/posts/vmware-tanzu.png
excerpt: The next Velero release (v1.2) will be built out of a new GH organization, and we have significant changes to our plugins.
author_name: Carlisia Campos
author_avatar: /img/contributors/carlisia-campos.png
categories: ['velero']
# Tag should match author to drive author pages
tags: ['Carlisia Campos']
---

## VMware Tanzu

Back in November 2018, the [VMWare acquisition of Heptio][1] made the news and in December we, the Heptio teams, started on our journey of integrating into their fold. Back then, our Velero project was named Heptio Ark and, in February, our v0.11 release brought about [our new name][2].

Nine months later, we come full circle with our latest and very exciting change: we are now part of a brand new GitHub organization: [VMware Tanzu][3]. VMware Tanzu is a new family of products and services for the cloud native world. With the Velero project being a cloud native technology that extends Kubernetes and is a member of the CNCF (Cloud Native Computing Foundation), it is only natural that it would be moved to sit alongside all of the VMware supported cloud native repositories. You can read more about this organization wide change in this [VMware blog post][4].

## The new Velero

The new Velero repository now can be found at https://github.com/vmware-tanzu/velero.

Items to note:</br>
* Past issues, PRs, commits, contributors, etc all have been moved to this repo</br>
* The Velero images will continue to be hosted at https://gcr.io/heptio-images

The next Velero release, version 1.2, will be built out of this new repository and is slated to come out at the end of October. The main [set of changes][5] for v1.2 is the restructuring around how we will be handling **all** `object store` and `volume snapshotter` plugins. Previously, Velero natively supported both types of plugins for AWS, Azure, and GCP (Google Cloud Platform). Going forward as of v1.2, this will no longer be the case.

## Velero plugins

With more and more providers wanting to support Velero, it gets more difficult to justify excluding those from being in-tree alongside the three original ones (AWS, Azure, GCP). At the same time, if we were to include any more plugins in-tree, it would ultimately become the responsibility of the Velero team to maintain an increasing number of plugins. As the opportunity to move to a new GitHub organization presented itself, we thought it was a good time to make some changes.

The three original native plugins and their respective documentation now each have their own repo under the new VMware-Tanzu GitHub organization. These are:

* [AWS][6]
* [Azure][7]
* [GCP][8]

Maintenance of these plugins will continue to be done by the Velero core team as usual, although we will gladly promote active contributors to maintainers. This change mainly aims to achieve these goals:

* equalize the field so all plugins are treated equally
* encourage developers to get involved with the smaller code base and potentially be promoted to maintainers
* iterating on plugins separately from the core codebase
* smaller binaries/images with the SDK for only the provider(s) needed

Instructions for how to [install Velero and plugins][9] or for how to [upgrade to v1.2][10] can be found in our documentation.

## Feedback

As always, we welcome feedback and participation in the development of Velero. All information on how to contact us or become active can be found here: https://velero.io/community/


[1]: https://blog.heptio.com/heptio-will-be-joining-forces-with-vmware-on-a-shared-cloud-native-mission-b01225b1bc9e
[2]: https://blogs.vmware.com/cloudnative/2019/02/28/velero-v0-11-delivers-an-open-source-tool-to-back-up-and-migrate-kubernetes-clusters/
[3]: https://github.com/vmware-tanzu
[4]: todo:addblogpost
[5]: https://github.com/heptio/velero/issues#workspaces/velero-5c59c15e39d47b774b5864e3/board?milestones=v1.2%232019-10-31&filterLogic=any&repos=99143276&showPipelineDescriptions=false
[6]: https://github.com/vmware-tanzu/velero-plugin-aws
[7]: https://github.com/vmware-tanzu/velero-plugin-azure
[8]: https://github.com/vmware-tanzu/velero-plugin-gcp
[9]: addlink
[10]: addlink

<!-- todo: correct the address for the new zenhub URL, link [5] -->