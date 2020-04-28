# Running in Docker

To run this site in a Docker container, you can use `make serve-docs` from the root directory.

# Dependencies for MacOS

Install the following for an easy to use dev environment:

* `brew install rbenv`
* `rbenv install 2.6.3`
* `gem install bundler`

# Dependencies for Linux
If you are running a build on Ubuntu you will need the following packages:
* ruby
* ruby-dev
* ruby-bundler
* build-essential
* zlib1g-dev
* nginx (or apache2)


# Local Development
1. Install Jekyll and plug-ins in one fell swoop. `gem install github-pages`
This mirrors the plug-ins used by GitHub Pages on your local machine including Jekyll, Sass, etc.
2. Clone down your own fork, or clone the main repo `git clone https://github.com/vmware-tanzu/velero` and add your own remote.
3. `cd velero/site`
4. `rbenv local 2.6.3`
5. `bundle install`
6. Serve the site and watch for markup/sass changes `jekyll serve --livereload --incremental`. You may need to run `bundle exec jekyll serve --livereload --incremental`.
7. View your website at http://127.0.0.1:4000/
8. Commit any changes and push everything to your fork.
9. Once you're ready, submit a PR of your changes. Netlify will automatically generate a preview of your changes.


# Adding a New Docs Version

To add a new set of versioned docs to go with a new Velero release:

1. In the root of the repository, run:

   ```bash
   # set to the appropriate version numbers
   NEW_DOCS_VERSION=vX.Y VELERO_VERSION=vX.Y.Z make gen-docs
   ```

1. In `site/_config.yml`, under the `defaults` field, add an entry for the new version just under `master` by copying the most recent version's entry and updating the version numbers.

1. [Pre-release only] In `site/_config.yml`, revert the change to the `latest` field, so the pre-release docs do not become the default.
