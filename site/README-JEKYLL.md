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
2. Clone down your own fork, or clone the main repo `git clone https://github.com/heptio/velero` and add your own remote.
3. `cd velero/site`
4. `rbenv local 2.6.3`
5. `bundle install`
6. Serve the site and watch for markup/sass changes `jekyll serve --livereload`. You may need to run `bundle exec jekyll serve --livereload`.
7. View your website at http://127.0.0.1:4000/
8. Commit any changes and push everything to your fork.
9. Once you're ready, submit a PR of your changes. Netlify will automatically generate a preview of your changes.
