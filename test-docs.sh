#!/bin/bash
trap cleanup INT TERM HUP EXIT

cleanup() {
  printf "Removing temporary Jekyll site...\n"
  rm -rf ./tmp
  printf "Doc testing complete!\n"
  ! (( $build_err || $link_err || $style_err )); exit $?
}

# Set up logs directory for script output
if [[ -z "$LOG_DIR" ]]; then
  mkdir -p $(pwd)/logs
  export LOG_DIR=$(pwd)/logs
fi

# Build the site (for the link checker)
printf "Building temporary Jekyll site (for testing)...\n"
start_time=`date +%s`
output_file=site-build.log
jekyll build --destination ./tmp/$PROJ_NAME &> $LOG_DIR/$output_file
build_err=$?
if [ "$build_err" -ne 0 ]; then
  printf "[ERROR] Check ./logs/$output_file for detailed logs.\n"
fi
printf "Done with build ($(expr `date +%s` - $start_time) s)!\n\n"

# Run the link checker
printf "Checking that links are valid in generated HTML...\n"
start_time=`date +%s`
output_file=link-check.log
$EXEC_DIR/htmlproofer \
  --assume-extension \
  --allow-hash-href \
  --empty-alt-ignore \
  ./tmp \
  &> $LOG_DIR/$output_file
link_err=$?
if [ "$link_err" -ne 0 ]; then
  printf "[ERROR] Check ./logs/$output_file for detailed logs.\n"
fi
printf "Done with link checker ($(expr `date +%s` - $start_time) s)!\n\n"

# Run the style checker
printf "Checking markdown (master branch) for appropriate (doc) style conventions...\n"
start_time=`date +%s`
output_file=style-check.log
$EXEC_DIR/vale ./master/*.md &> $LOG_DIR/$output_file
style_err=$?
if [ "$style_err" -ne 0 ]; then
  printf "[ERROR] Check ./logs/$output_file for detailed logs.\n"
fi
printf "Done with style checker ($(expr `date +%s` - $start_time) s)!\n\n"