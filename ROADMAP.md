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

`Last Updated: May 2020`

|Theme|Description|Timeline|
|--|--|--|
|Restic Improvements|Introduce improvements in annotating resources for Restic backup|August 2020|
|Extensibility|Add restore hooks for enhanced recovery scenarios|August 2020|
|CSI|Continue improving the CSI snapshot capabilities and participate in the upstream K8s CSI community|Long running (dependent on CSI working group)|
|Backup/Restore|Improvements to long-running copy operations from a performance and reliability standpoint|August 2020|
|Quality/Reliability| Enable automated end-to-end testing |Dec 2020|
|UX|Improvements to install and configuration user experience|Dec 2020|
|Restic Improvements|Improve the use of Restic in Velero and offer stable support|Dec 2020|
|Perf & Scale|Introduce a scalable model by using a worker pod for each backup/restore operation and improve operations|Dec 2020|
|Backup/Restore|Better backup and restore semantics for certain Kubernetes resources like stateful sets, operators|Dec 2020|
|Security|Enable the use of custom credential providers|Dec 2020|
|Self-Service & Multitenancy|Reduce friction by enabling developers to backup their namespaces via self-service. Introduce a Velero multi-tenancy model, enabling owners of namespaces to backup and restore within their access scope|Mar 2021|
|Backup/Restore|Cross availability zone or region backup and restore|Mar 2021|
|Application Consistency|Offer blueprints for backing up and restoring popular applications|May 2021|
|Backup/Restore|Data only backup and restore|May 2021|
|Backup/Restore|Introduce the ability to overwrite existing objects during a restore|May 2021|
|Backup/Restore|What-if dry run for backup and restore|May 2021|
