# Security

## Reporting a vulnerability

Please report security issues privately through
[GitHub security advisories](https://github.com/mjovanovic0/openshift-etcd-backup-explorer/security/advisories/new)
rather than opening a public issue. You should get a first response within a
week.

## What this tool does with your data

This matters more than usual here, so it is worth being explicit.

**An etcd backup contains every Secret in the cluster.** Unless the cluster
enabled [encryption at rest](https://docs.openshift.com/container-platform/latest/security/encrypting-etcd.html),
those Secrets sit in the snapshot as plain base64, which is not encryption.
Service account tokens, image pull credentials, TLS private keys and anything
an application stored in a Secret are all readable. Treat a snapshot file with
the same care as the cluster's root credentials.

Given that:

- **The snapshot is opened read only.** The file on disk is never modified.
- **Nothing leaves your machine.** There is no telemetry, no update check and
  no outbound network call of any kind. The only network listener is the local
  web server you start yourself.
- **The server binds to `127.0.0.1` by default.** There is no authentication,
  so if you change `-addr` to a public interface you are publishing every
  Secret in the backup to anyone who can reach that port. Do not do this on a
  shared or untrusted network.
- **Exported files are written mode `0600`,** readable only by the user who
  ran the export.
- **Browser storage holds preferences only.** Page size, font size and the
  export setting live in `localStorage`; no object data is stored there.

## Supported versions

Fixes go onto the latest release. This is a small project with no long term
support branches.
