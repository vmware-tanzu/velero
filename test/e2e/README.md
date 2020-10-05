# End-to-end tests
# Executing tests
## Using ginkgo locally
From the velero directory:
hack/tools/bin/ginkgo test/e2e

Command line flags can be set after --
velerocli - the velero CLI to use\
kibishiins - the namespace to install kibishii in\

hack/tools/bin/ginkgo test/e2e/ -- -velerocli=/usr/bin/velero
