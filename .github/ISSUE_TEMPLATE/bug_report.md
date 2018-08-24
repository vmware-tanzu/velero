---
name: Bug report
about: Tell us about a problem you are experiencing

---

**What steps did you take and what happened:**
[A clear and concise description of what the bug is, and what commands you ran.)


**What did you expect to happen:**


**The output of the following commands will help us better understand what's going on**:
(Pasting long output into a [GitHub gist](https://gist.github.com) or other pastebin is fine.)

* `kubectl logs deployment/ark -n heptio-ark`
* `ark backup describe <backupname>` or `kubectl get backup/<backupname> -n heptio-ark -o yaml`
* `ark backup logs <backupname>`
* `ark restore describe <restorename>` or `kubectl get restore/<restorename> -n heptio-ark -o yaml`
* `ark restore logs <restorename>`


**Anything else you would like to add:**
[Miscellaneous information that will assist in solving the issue.]


**Environment:**

- Ark version (use `ark version`):  
- Kubernetes version (use `kubectl version`):
- Kubernetes installer & version:
- Cloud provider or hardware configuration:
- OS (e.g. from `/etc/os-release`):
