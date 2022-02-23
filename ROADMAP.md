## Velero Roadmap

### About this document
This document provides a link to the [Velero Project boards](https://github.com/vmware-tanzu/velero/projects) that serves as the up to date description of items that are in the release pipeline. The release boards have separate swim lanes based on prioritization. Most items are gathered from the community or include a feedback loop with the community. This should serve as a reference point for Velero users and contributors to understand where the project is heading, and help determine if a contribution could be conflicting with a longer term plan. 

### How to help?
Discussion on the roadmap can take place in threads under [Issues](https://github.com/vmware-tanzu/velero/issues) or in [community meetings](https://velero.io/community/). Please open and comment on an issue if you want to provide suggestions, use cases, and feedback to an item in the roadmap. Please review the roadmap to avoid potential duplicated effort.

### How to add an item to the roadmap?
One of the most important aspects in any open source community is the concept of proposals. Large changes to the codebase and / or new features should be preceded by a [proposal](https://github.com/vmware-tanzu/velero/blob/main/GOVERNANCE.md#proposal-process) in our repo.
For smaller enhancements, you can open an issue to track that initiative or feature request.
We work with and rely on community feedback to focus our efforts to improve Velero and maintain a healthy roadmap.

### Current Roadmap
The following table includes the current roadmap for Velero. If you have any questions or would like to contribute to Velero, please attend a [community meeting](https://velero.io/community/) to discuss with our team. If you don't know where to start, we are always looking for contributors that will help us reduce technical, automation, and documentation debt.
Please take the timelines & dates as proposals and goals. Priorities and requirements change based on community feedback, roadblocks encountered, community contributions, etc. If you depend on a specific item, we encourage you to attend community meetings to get updated status information, or help us deliver that feature by contributing to Velero.

`Last Updated: October 2021`

#### 1.8.0 Roadmap (to be delivered January/February 2021)

|Issue|Description|Timeline|Notes|
|---|---|---|---|
|[4108](https://github.com/vmware-tanzu/velero/issues/4108), [4109](https://github.com/vmware-tanzu/velero/issues/4109)|Solution for CSI - Azure and AWS|2022 H1|Currently, Velero plugins for AWS and Azure cannot back up persistent volumes that were provisioned using the CSI driver. This will fix that.|
|[3229](https://github.com/vmware-tanzu/velero/issues/3229),[4112](https://github.com/vmware-tanzu/velero/issues/4112)|Moving data mover functionality from the Velero Plugin for vSphere into Velero proper|2022 H1|This work is a precursor to decoupling the Astrolabe snapshotting infrastructure.|
|[3533](https://github.com/vmware-tanzu/velero/issues/3533)|Upload Progress Monitoring|2022 H1|Finishing up the work done in the 1.7 timeframe. The data mover work depends on this.|
|[1975](https://github.com/vmware-tanzu/velero/issues/1975)|Test dual stack mode|2022 H1|We already tested IPv6, but we want to confirm that dual stack mode works as well.|
|[2082](https://github.com/vmware-tanzu/velero/issues/2082)|Delete Backup CRs on removing target location. |2022 H1||
|[3516](https://github.com/vmware-tanzu/velero/issues/3516)|Restore issue with MutatingWebhookConfiguration v1beta1 API version|2022 H1||
|[2308](https://github.com/vmware-tanzu/velero/issues/2308)|Restoring nodePort service that has nodePort preservation always fails if service already exists in the namespace|2022 H1||
|[4115](https://github.com/vmware-tanzu/velero/issues/4115)|Support for multiple set of credentials for VolumeSnapshotLocations|2022 H1||
|[1980](https://github.com/vmware-tanzu/velero/issues/1980)|Velero triggers backup immediately for scheduled backups|2022 H1||
|[4067](https://github.com/vmware-tanzu/velero/issues/4067)|Pre and post backup and restore hooks|2022 H1||
|[3742](https://github.com/vmware-tanzu/velero/issues/3742)|Carvel packaging for Velero for vSphere|2022 H1|AWS and Azure have been completed already.|
|[3285](https://github.com/vmware-tanzu/velero/issues/3285)|Design doc for Velero plugin versioning|2022 H1||
|[4231](https://github.com/vmware-tanzu/velero/issues/4231)|Technical health (prioritizing giving developers confidence and saving developers time)|2022 H1|More automated tests (especially the pre-release manual tests) and more automation of the running of tests.|
|[4110](https://github.com/vmware-tanzu/velero/issues/4110)|Solution for CSI - GCP|2022 H1|Currently, the Velero plugin for GCP cannot back up persistent volumes that were provisioned using the CSI driver. This will fix that.|
|[3742](https://github.com/vmware-tanzu/velero/issues/3742)|Carvel packaging for Velero for restic|2022 H1|AWS and Azure have been completed already.|
|[3454](https://github.com/vmware-tanzu/velero/issues/3454),[4134](https://github.com/vmware-tanzu/velero/issues/4134),[4135](https://github.com/vmware-tanzu/velero/issues/4135)|Kubebuilder tech debt|2022 H1||
|[4111](https://github.com/vmware-tanzu/velero/issues/4111)|Ignore items returned by ItemSnapshotter.AlsoHandles during backup|2022 H1|This will enable backup of complex objects, because we can then tell Velero to ignore things that were already backed up when Velero was previously called recursively.|

Other work may make it into the 1.8 release, but this is the work that will be prioritized first.