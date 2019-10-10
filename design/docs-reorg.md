# Documentation Reorganization

One to two sentences that describes the goal of this proposal.
The reader should be able to tell by the title, and the opening paragraph, if this document is relevant to them.

_Note_: The preferred style for design documents is one sentence per line.
*Do not wrap lines*.
This aids in review of the document as changes to a line are not obscured by the reflowing those changes caused and has a side effect of avoiding debate about one or two space after a period.

## Goals

- A short list of things which will be accomplished by implementing this proposal.
- Two things is ok.
- Three is pushing it.
- More than three goals suggests that the proposal's scope is too large.

## Non Goals

- A short list of items which are:
- a. out of scope
- b. follow on items which are deliberately excluded from this proposal.

## Background

One to two paragraphs of exposition to set the context for this proposal.

## High-Level Design

- Concepts
    - Overview
        - Core Features
        - Architecture
    - Storage Providers/Plugins
    - Backup Storage Locations
    - Volume Snapshot Locations
    - Restic Integration
- Setup
    - Overview (outline of install process, with links)
        - Review the requirements
            - Kubernetes version
            - Compatible object storage
            - (Optional) compatible block storage
            - Resource requirements
        - Download the Velero release
        - Install on your platform
    - Supported Storage Providers
    - Evaluation Install (using in-cluster MinIO)
    - Restic setup
    - Uninstalling
    - `velero install` reference
        - cover some of the most common flags
- Use (TODO add some subsections for organization)
    - Install sample workload
    - Take a backup
    - Create a scheduled backup
    - Recover a namespace
    - Recover from full-cluster failure
    - Clone a namespace
    - Migrate to a new cluster
    - Run velero in a different namespace
    - Configure a hook
    ...
- Contribute
    - Start Contributing
    - ZenHub
    - Development
    - Build from source
    - Run locally
    - Plugin development
- Reference
    - Backup file format
    - API types reference

To be determined:
- FAQ?
- Troubleshooting?
