---
title: "Support Process"
layout: docs
---

## Weekly Rotation

The Velero maintainers use a weekly rotation to manage community support. Each week, a different maintainer is the point person for responding to incoming support issues via Slack, GitHub, and the Google group. The point person is *not* expected to be on-call 24x7. Instead, they choose one or more hour(s) per day to be available/responding to incoming issues. They will communicate to the community what that time slot will be each week.

## Start of Week

We will update the public Slack channel's topic to indicate that you are the point person for the week, and what hours you'll be available.

## During the Week

### Where we will monitor
- `#velero` public Slack channel in Kubernetes org
- [all Velero-related repos][0] in GitHub (`velero`, `velero-plugin-for-[aws|gcp|microsoft-azure|csi]`, `helm-charts`)
- [Project Velero Google Group][1]

### GitHub issue flow

Generally speaking, new GitHub issues will fall into one of several categories. We use the following process for each:

1. **Feature request**
    - Label the issue with `Enhancement/User` or `Enhancement/Dev`
    - Leave the issue in the `New Issues` swimlane for triage by product mgmt
1. **Bug**
    - Label the issue with `Bug`
    - Leave the issue in the `New Issues` swimlane for triage by product mgmt
1. **User question/problem** that does not clearly fall into one of the previous categories
    - When you start investigating/responding, label the issue with `Investigating`
    - Add comments as you go, so both the user and future support people have as much context as possible
    - Use the `Needs Info` label to indicate an issue is waiting for information from the user. Remove/re-add the label as needed.
    - If you resolve the issue with the user, close it out
    - If the issue ends up being a feature request or a bug, update the title and follow the appropriate process for it
    - If the reporter becomes unresponsive after multiple pings, close out the issue due to inactivity and comment that the user can always reach out again as needed
    
## End of Week

We ensure all GitHub issues worked on during the week on are labeled with `Investigating` and `Needs Info` (if appropriate), and have updated comments so the next person can pick them up.

[0]: https://app.zenhub.com/workspaces/velero-5c59c15e39d47b774b5864e3/board?repos=99143276,112385197,213946861,190224441,214524700,214524630
[1]: https://groups.google.com/forum/#!forum/projectvelero
