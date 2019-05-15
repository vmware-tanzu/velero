# Dependencies
If you are running a build on Ubuntu you will need the following packages 
* ruby
* ruby-dev
* ruby-bundler
* build-essential
* zlib1g-dev
* nginx (or apache2)

For other operating systems such as MacOS you will need equivalent packages or install xcode

# Local Development
1. Install Jekyll and plug-ins in one fell swoop. `gem install github-pages` 
This mirrors the plug-ins used by GitHub Pages on your local machine including Jekyll, Sass, etc.
2. Clone down your fork `git@github.com:smalltalk-ai/vmware-jekyll-velero.git`
3. cd into the `site` directory
4. Serve the site and watch for markup/sass changes `jekyll serve --livereload`. you may need to run `bundle exec jekyll serve --livereload`.
5. View your website at http://127.0.0.1:4000/
6. Commit any changes and push everything to the master branch of your GitHub user repository. GitHub Pages will then rebuild and serve your website.