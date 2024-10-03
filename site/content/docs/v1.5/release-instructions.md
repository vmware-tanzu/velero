---
title: "Release Instructions"
layout: docs
toc: "true"
---
This page covers the steps to perform when releasing a new version of Velero.

## General notes
- Please read the documented variables in each script to understand what they are for and how to properly format their values.
- You will need to have a remote named `upstream` pointing to the Velero repository so the scripts can work properly.
- GA release: major and minor releases only. Example: 1.0 (major), 1.5 (minor).
- Pre-releases: Any release leading up to a GA. Example: 1.4.0-beta.1, 1.5.0-rc.1
- RC releases: Release Candidate, contains everything that is supposed to ship with the GA release. This is still a pre-release.

## Preparing

### Create release blog post (GA only)
For each major or minor release, create and publish a blog post to let folks know what's new. Please follow these [instructions](how-to-write-and-release-a-blog-post).

### Changelog and Docs PR
#### Troubleshooting
- If you encounter the error `You don't have enough free space in /var/cache/apt/archives/` when running `make serve-docs`: run `docker system prune`.

#### Steps
1.  If it doesn't already exist: in a branch, create the file `changelogs/CHANGELOG-<major>.<minor>.md` by copying the most recent one.
1.  Update the file `changelogs/CHANGELOG-<major>.<minor>.md`
	- Run `make changelog` to generate a list of all unreleased changes.
    - Copy/paste the output into `CHANGELOG-<major>.<minor>.md`, under the "All Changes" section for the release. 
	- You *may* choose to tweak formatting on the list of changes by adding code blocks, etc.
	- 	Update links at the top of the file to point to the new release version
1.  Update the main `CHANGELOG.md` file to properly reference the release-specific changelog file
	- Under "Current release": 
	    - Should contain only the current GA release.
    - Under "Development release": 
	    - Should contain only the latest pre-release
	    - Move any prior pre-release into "Older releases"
1. GA Only: Remove all changelog files from `changelogs/unreleased`.
1. Generate new docs
	- Run `make gen-docs`, passing the appropriate variables. Examples:
		a) `VELERO_VERSION=v1.5.0-rc.1 NEW_DOCS_VERSION=v1.5.0-rc.1 make gen-docs`.
		b) `VELERO_VERSION=v1.5.0 NEW_DOCS_VERSION=v1.5 make gen-docs`).
	- Note: `PREVIOUS_DOCS_VERSION=<doc-version-to-copy-from>` is optional; when not set, it will default to the latest doc version.
1. Clean up when there is an existing set of pre-release versioned docs for the version you are releasing
	- Example: `site/content/docs/v1.5.0-beta.1` exists, and you're releasing `v1.5.0-rc.1` or `v1.5`
    - Remove the directory containing the pre-release docs, i.e. `site/content/docs/<pre-release-version>`.
    - Delete the pre-release docs table of contents file, i.e. `site/data/docs/<pre-release-version>-toc.yml`.
    - Remove the pre-release docs table of contents mapping entry from `site/data/toc-mapping.yml`.
    - Remove all references to the pre-release docs from `site/config.yml`.
1. Create the "Upgrade to $major.minor" page if it does not already exist ([example](https://velero.io/docs/v1.5/upgrade-to-1.5/)).
   If it already exists, update any usage of the previous version string within this file to use the new version string instead ([example](https://github.com/vmware-tanzu/velero/pull/2941/files#diff-d594f8fd0901fed79c39aab4b348193d)).
   This needs to be done in both the versioned and the `main` folders.
1. Review and submit PR
	- Follow the additional instructions at `site/README-HUGO.md` to complete the docs generation process.
	- Do a review of the diffs, and/or run `make serve-docs` and review the site.
	- Submit a PR containing the changelog and the version-tagged docs.

## Velero release
### Notes
- Pre-requisite: PR with the changelog and docs is merged, so that it's included in the release tag.
- This process is the same for both pre-release and GA.
- Refer to the [General notes](general-notes) above for instructions.

#### Troubleshooting
- If the dry-run fails with random errors, try running it again.

#### Steps
1.  Create a tagged release in dry-run mode
	- This won't push anything to GitHub.
	- Run `VELERO_VERSION=v1.0.0-rc.1 GITHUB_TOKEN=REDACTED ./hack/release-tools/tag-release.sh`.
	- Fix any issue.
1. Create a tagged release and push it to GitHub
	- Run `VELERO_VERSION=v1.0.0-rc.1 GITHUB_TOKEN=REDACTED ./hack/release-tools/tag-release.sh publish`.
1. Publish the release
	- Navigate to the draft GitHub release at https://github.com/vmware-tanzu/velero/releases and edit the release.
	- If this is a patch release (e.g. `v1.4.1`), note that the full `CHANGELOG-1.4.md` contents will be included in the body of the GitHub release. You need to delete the previous releases' content (e.g. `v1.2.0`'s changelog) so that only the latest patch release's changelog shows.
	- Do a quick review for formatting. 
	- **Note:** the `goreleaser` process should have detected if it's a pre-release version and, if so, checked the box at the bottom of the GitHub release page appropriately, but it's always worth double-checking.
	- Verify that GitHub has built and pushed all the images (it takes a while): https://github.com/vmware-tanzu/velero/actions
	- Verify that the images are on Docker Hub: https://hub.docker.com/r/velero/velero/tags
	- Verify that the assets were published to the GitHub release
	- Publish the release.
1.  Test the release
	- By now, the Docker images should have been published. 
	- Perform a smoke-test - for example:
		- Download the CLI from the GitHub release
	    - Use it to install Velero into a cluster (or manually update an existing deployment to use the new images)
	    - Verify that `velero version` shows the expected output
	    - Run a backup/restore and ensure it works

## Homebrew release (GA only)
These are the steps to update the Velero Homebrew version.

### Steps
- If you don't already have one, create a [GitHub access token for Homebrew](https://github.com/settings/tokens/new?scopes=gist,public_repo&description=Homebrew)
- Run `export HOMEBREW_GITHUB_API_TOKEN=your_token_here` on your command line to make sure that `brew` can work on GitHub on your behalf.
- Run `hack/release-tools/brew-update.sh`. This script will download the necessary files, do the checks, and invoke the brew helper to submit the PR, which will open in your browser.
- Update Windows Chocolatey version. From a Windows computer, follow the step-by-step instructions to [create the Windows Chocolatey package for Velero CLI](https://github.com/adamrushuk/velero-choco/blob/main/README.md)

## How to write and release a blog post
What to include in a release blog:
* Thank all contributors for their involvement in the release.
  * Where possible shoutout folks by name or consider spotlighting new maintainers.
* Highlight the themes, or areas of focus, for the release. Some examples of themes are security, bug fixes, feature improvements. See past Velero [release blog posts][1] for more examples.
* Include summaries of new features or workflows introduced in a release.
  * This can also include new project initiatives, like a code-of-conduct update.
  * Consider creating additional blog posts that go through new features in more detail. Plan to publish additional blogs after the release blog (all blogs don’t have to be publish all at once).

Release blog post PR:
* Prepare a PR containing the release blog post. Read the [website guidelines][2] for more information on creating a blog post. It's usually easiest to make a copy of the most recent existing post, then replace the content as appropriate.
* You also need to update `site/index.html` to have "Latest Release Information" contain a link to the new post.
* Plan to publish the blog post the same day as the release.

## Announce a release
Once you are finished doing the release, let the rest of the world know it's available by posting messages in the following places.
1.  GA Only: Merge the blog post PR.
1. Velero's Twitter account. Maintainers are encouraged to help spread the word by posting or reposting on social media.
1. Community Slack channel.
1. Google group message.

What to include:
* Thank all contributors
* A brief list of highlights in the release
* Link to the release blog post, release notes, and/or github release page

[1]: https://velero.io/blog
[2]: website-guidelines.md
