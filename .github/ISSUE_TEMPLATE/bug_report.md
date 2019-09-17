---
name: Bug report
about: Tell us about a problem you are experiencing

---

**What steps did you take and what happened:**
[A clear and concise description of what the bug is, and what commands you ran.)


**What did you expect to happen:**


**The output of the following commands will help us better understand what's going on**:
(Pasting long output into a [GitHub gist](https://gist.github.com) or other pastebin is fine.)

* `kubectl logs deployment/velero -n velero`
* `velero backup describe <backupname>` or `kubectl get backup/<backupname> -n velero -o yaml`
* `velero backup logs <backupname>`
* `velero restore describe <restorename>` or `kubectl get restore/<restorename> -n velero -o yaml`
* `velero restore logs <restorename>`


**Anything else you would like to add:**
[Miscellaneous information that will assist in solving the issue.]


**Environment:**

- Velero version (use `velero version`): 
- Velero features (use `velero client config get features`): 
- Kubernetes version (use `kubectl version`):
- Kubernetes installer & version:
- Cloud provider or hardware configuration:
- OS (e.g. from `/etc/os-release`):
