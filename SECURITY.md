# Security Release Process

Velero is an open source tool with a growing community devoted to safe backup and restore, disaster recovery, and data migration of Kubernetes resources and persistent volumes. The community has adopted this security disclosure and response policy to ensure we responsibly handle critical issues.


## Supported Versions

The Velero project maintains the following [governance document](https://github.com/vmware-tanzu/velero/blob/main/GOVERNANCE.md), [release document](https://github.com/vmware-tanzu/velero/blob/f42c63af1b9af445e38f78a7256b1c48ef79c10e/site/docs/main/release-instructions.md), and [support document](https://velero.io/docs/main/support-process/). Please refer to these for release and related details. Only the most recent version of Velero is supported. Each [release](https://github.com/vmware-tanzu/velero/releases) includes information about upgrading to the latest version.


## Reporting a Vulnerability - Private Disclosure Process

Security is of the highest importance and all security vulnerabilities or suspected security vulnerabilities should be reported to Velero privately, to minimize attacks against current users of Velero before they are fixed. Vulnerabilities will be investigated and patched on the next patch (or minor) release as soon as possible. This information could be kept entirely internal to the project.  

If you know of a publicly disclosed security vulnerability for Velero, please **IMMEDIATELY** contact the VMware Security Team (security@vmware.com).

 

**IMPORTANT: Do not file public issues on GitHub for security vulnerabilities**

To report a vulnerability or a security-related issue, please contact the VMware email address with the details of the vulnerability. The email will be fielded by the VMware Security Team and then shared with the Velero maintainers who have committer and release permissions. Emails will be addressed within 3 business days, including a detailed plan to investigate the issue and any potential workarounds to perform in the meantime. Do not report non-security-impacting bugs through this channel. Use [GitHub issues](https://github.com/vmware-tanzu/velero/issues/new/choose) instead.


## Proposed Email Content

Provide a descriptive subject line and in the body of the email include the following information:



*   Basic identity information, such as your name and your affiliation or company.
*   Detailed steps to reproduce the vulnerability  (POC scripts, screenshots, and logs are all helpful to us).
*   Description of the effects of the vulnerability on Velero and the related hardware and software configurations, so that the VMware Security Team can reproduce it.
*   How the vulnerability affects Velero usage and an estimation of the attack surface, if there is one.
*   List other projects or dependencies that were used in conjunction with Velero to produce the vulnerability.

 


## When to report a vulnerability



*   When you think Velero has a potential security vulnerability.
*   When you suspect a potential vulnerability but you are unsure that it impacts Velero.
*   When you know of or suspect a potential vulnerability on another project that is used by Velero.

  


## Patch, Release, and Disclosure

The VMware Security Team will respond to vulnerability reports as follows:

 



1. The Security Team will investigate the vulnerability and determine its effects and criticality.
2. If the issue is not deemed to be a vulnerability, the Security Team will follow up with a detailed reason for rejection.
3. The Security Team will initiate a conversation with the reporter within 3 business days.
4. If a vulnerability is acknowledged and the timeline for a fix is determined, the Security Team will work on a plan to communicate with the appropriate community, including identifying mitigating steps that affected users can take to protect themselves until the fix is rolled out.
5. The Security Team will also create a [CVSS](https://www.first.org/cvss/specification-document) using the [CVSS Calculator](https://www.first.org/cvss/calculator/3.0). The Security Team makes the final call on the calculated CVSS; it is better to move quickly than making the CVSS perfect. Issues may also be reported to [Mitre](https://cve.mitre.org/) using this [scoring calculator](https://nvd.nist.gov/vuln-metrics/cvss/v3-calculator). The CVE will initially be set to private.
6. The Security Team will work on fixing the vulnerability and perform internal testing before preparing to roll out the fix.
7. The Security Team will provide early disclosure of the vulnerability by emailing the [Velero Distributors](https://groups.google.com/u/1/g/projectvelero-distributors) mailing list. Distributors can initially plan for the vulnerability patch ahead of the fix, and later can test the fix and provide feedback to the Velero team. See the section **Early Disclosure to Velero Distributors List** for details about how to join this mailing list. 
8. A public disclosure date is negotiated by the VMware SecurityTeam, the bug submitter, and the distributors list. We prefer to fully disclose the bug as soon as possible once a user mitigation or patch is available. It is reasonable to delay disclosure when the bug or the fix is not yet fully understood, the solution is not well-tested, or for distributor coordination. The timeframe for disclosure is from immediate (especially if it’s already publicly known) to a few weeks. For a critical vulnerability with a straightforward mitigation, we expect the report date for the public disclosure date to be on the order of 14 business days. The VMware Security Team holds the final say when setting a public disclosure date.
9. Once the fix is confirmed, the Security Team will patch the vulnerability in the next patch or minor release, and backport a patch release into all earlier supported releases. Upon release of the patched version of Velero, we will follow the **Public Disclosure Process**.


## Public Disclosure Process

The Security Team publishes a [public advisory](https://github.com/vmware-tanzu/velero/security/advisories) to the Velero community via GitHub. In most cases, additional communication via Slack, Twitter, mailing lists, blog and other channels will assist in educating Velero users and rolling out the patched release to affected users. 

The Security Team will also publish any mitigating steps users can take until the fix can be applied to their Velero instances. Velero distributors will handle creating and publishing their own security advisories.

 


## Mailing lists



*   Use security@vmware.com to report security concerns to the VMware Security Team, who uses the list to privately discuss security issues and fixes prior to disclosure.
*   Join the [Velero Distributors](https://groups.google.com/u/1/g/projectvelero-distributors) mailing list for early private information and vulnerability disclosure. Early disclosure may include mitigating steps and additional information on security patch releases. See below for information on how Velero distributors or vendors can apply to join this list.


## Early Disclosure to Velero Distributors List

The private list is intended to be used primarily to provide actionable information to multiple distributor projects at once. This list is not intended to inform individuals about security issues. 


## Membership Criteria

To be eligible to join the [Velero Distributors](https://groups.google.com/u/1/g/projectvelero-distributors) mailing list, you should:



1. Be an active distributor of Velero.
2. Have a user base that is not limited to your own organization.
3. Have a publicly verifiable track record up to the present day of fixing security issues.
4. Not be a downstream or rebuild of another distributor.
5. Be a participant and active contributor in the Velero community.
6. Accept the Embargo Policy that is outlined below. 
7. Have someone who is already on the list vouch for the person requesting membership on behalf of your distribution.

**The terms and conditions of the Embargo Policy apply to all members of this mailing list. A request for membership represents your acceptance to the terms and conditions of the Embargo Policy.**


## Embargo Policy

The information that members receive on the Velero Distributors mailing list must not be made public, shared, or even hinted at anywhere beyond those who need to know within your specific team, unless you receive explicit approval to do so from the VMware Security Team. This remains true until the public disclosure date/time agreed upon by the list. Members of the list and others cannot use the information for any reason other than to get the issue fixed for your respective distribution's users.

Before you share any information from the list with members of your team who are required to fix the issue, these team members must agree to the same terms, and only be provided with information on a need-to-know basis.

In the unfortunate event that you share information beyond what is permitted by this policy, you must urgently inform the VMware Security Team (security@vmware.com) of exactly what information was leaked and to whom. If you continue to leak information and break the policy outlined here, you will be permanently removed from the list.

 


## Requesting to Join

Send new membership requests to projectvelero-distributors@googlegroups.com. In the body of your request please specify how you qualify for membership and fulfill each criterion listed in the Membership Criteria section above.


## Confidentiality, integrity and availability

We consider vulnerabilities leading to the compromise of data confidentiality, elevation of privilege, or integrity to be our highest priority concerns. Availability, in particular in areas relating to DoS and resource exhaustion, is also a serious security concern. The VMware Security Team takes all vulnerabilities, potential vulnerabilities, and suspected vulnerabilities seriously and will investigate them in an urgent and expeditious manner.

Note that we do not currently consider the default settings for Velero to be secure-by-default. It is necessary for operators to explicitly configure settings, role based access control, and other resource related features in Velero to provide a hardened Velero environment. We will not act on any security disclosure that relates to a lack of safe defaults. Over time, we will work towards improved safe-by-default configuration, taking into account backwards compatibility.
