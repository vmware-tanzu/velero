# Documentation Reorganization

This document proposes a new outline for the Velero documentation that is intended to be easier for both users and contributors to interact with.

## Goals

- Agree on the desired outline (sections and pages) for Velero documentation.

## Non Goals

- Write/re-write all new/revised pages (individual page writes/rewrites to be tracked as individual issues).

## Background

The Velero documentation has evolved organically since the project's inception.
While it contains a lot of useful information, it is not currently organized in a way that is easy to navigate for users or contributors.
For example, the introductory section jumps straight into a discussion of Velero's architecture, rather than starting with a good overview of its core features.
Also, the documentation on how to actually *use* Velero is fairly superficial, only covering the most straight-forward use cases.
Additionally, different types of information are often mixed together (i.e. conceptual overviews along with detailed setup instructions).

This design document proposes a new outline for the documentation that is intended to be more easily navigated by both users and contributors.
The new outline both reorganizes the existing content, *and* calls for new content where relevant.

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
