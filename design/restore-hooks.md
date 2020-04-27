# Restore Hooks

Velero supports Backup Hooks to execute commands at before and/or after a backup.
This enables a user to, among other things, prepare data to be backed up.
An example of this would be to attach an empty volume to a Postgres pod, use a backup hook to execute `pg_dump` from the data volume, and back up the volume containing the export.
The problem is that there's no easy or automated way to include an automated restore process.
After a restore with the example configuration above, the postgres pod will be empty, but there will be a need to manually exec in and run `pg_restore`.
This is not specific to postgres backups, but any many database engines and other applications that have application-specific utilities to back up and restore the data.
This document proposes a solution that allows a user to specify Restore Hooks, much like Backup Hooks, that can be executed during the restore process.

## Goals

- Enable custom commands to be run during a restore in order to mirror the commands that are available to the backup process.

## Non Goals

- Handling any application specific scenarios (postgres, mongo, etc)

## Background

(See introduction)

## High-Level Design

The Restore spec

## Detailed Design


## Alternatives Considered


## Security Considerations

N/A
