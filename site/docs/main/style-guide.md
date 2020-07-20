
# Documentation Style Guide

_This style guide is adapted from the [Kubernetes style guide](https://kubernetes.io/docs/contribute/style/style-guide/)._

This page outlines writing style guidelines for the Velero documentation and you should use this page as a reference you write or edit content. Note that these are guidelines, not rules. Use your best judgment as you write documentation, and feel free to propose changes to these guidelines. Changes to the style guide are made by the Velero maintainers as a group. To propose a change or addition create an issue/PR, or add a suggestion to the [community meeting agenda](https://hackmd.io/Jq6F5zqZR7S80CeDWUklkA) and attend the meeting to participate in the discussion.

The Velero documentation uses the [kramdown](https://kramdown.gettalong.org/) Markdown renderer.

## Content best practices


### Use present tense

<table caption="Do and Don't - Use present tense">
  <tr><th>Do</th><th>Don't</th></tr>
  <tr><td>This `command` starts a proxy.</td><td>This command will start a proxy.</td></tr>
</table>

Exception: Use future or past tense if it is required to convey the correct meaning.

### Use active voice

<table caption="Do and Don't - Use active voice" >
  <tr><th>Do</th><th>Don't</th></tr>
  <tr><td>You can explore the API using a browser.</td><td>The API can be explored using a browser.</td></tr>
  <tr><td>The YAML file specifies the replica count.</td><td>The replica count is specified in the YAML file.</td></tr>
</table>

Exception: Use passive voice if active voice leads to an awkward sentence construction.

### Use simple and direct language

Use simple and direct language. Avoid using unnecessary phrases, such as saying "please."

<table caption="Do and Don't - Use simple and direct language" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>To create a ReplicaSet, ...</td><td>In order to create a ReplicaSet, ...</td></tr>
<tr><td>See the configuration file.</td><td>Please see the configuration file.</td></tr>
<tr><td>View the Pods.</td><td>With this next command, we'll view the Pods.</td></tr>
</table>

### Address the reader as "you"

<table caption="Do and Don't - Addressing the reader" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>You can create a Deployment by ...</td><td>We'll create a Deployment by ...</td></tr>
<tr><td>In the preceding output, you can see...</td><td>In the preceding output, we can see ...</td></tr>
</table>

### Avoid Latin phrases

Prefer English terms over Latin abbreviations.

<table caption="Do and Don't - Avoid Latin phrases" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>For example, ...</td><td>e.g., ...</td></tr>
<tr><td>That is, ...</td><td>i.e., ...</td></tr>
</table>

Exception: Use "etc." for et cetera.

## Patterns to avoid


### Avoid using "we"

Using "we" in a sentence can be confusing, because the reader might not know
whether they're part of the "we" you're describing.

<table caption="Do and Don't - Patterns to avoid" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>Version 1.4 includes ...</td><td>In version 1.4, we have added ...</td></tr>
<tr><td>Kubernetes provides a new feature for ...</td><td>We provide a new feature ...</td></tr>
<tr><td>This page teaches you how to use Pods.</td><td>In this page, we are going to learn about Pods.</td></tr>
</table>

### Avoid jargon and idioms

Many readers speak English as a second language. Avoid jargon and idioms to help them understand better.

<table caption="Do and Don't - Avoid jargon and idioms" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>Internally, ...</td><td>Under the hood, ...</td></tr>
<tr><td>Create a new cluster.</td><td>Turn up a new cluster.</td></tr>
</table>

### Avoid statements about the future

Avoid making promises or giving hints about the future. If you need to talk about
a beta feature, put the text under a heading that identifies it as beta
information.

### Avoid statements that will soon be out of date

Avoid words like “recently”, "currently" and "new." A feature that is new today might not be
considered new in a few months.

<table caption="Do and Don't - Avoid statements that will soon be out of date" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>In version 1.4, ...</td><td>In the current version, ...</td></tr>
<tr><td>The Federation feature provides ...</td><td>The new Federation feature provides ...</td></tr>
</table>

### Language

This documentation uses U.S. English spelling and grammar.

## Documentation formatting standards


### Use camel case for API objects

When you refer to an API object, use the same uppercase and lowercase letters
that are used in the actual object name. Typically, the names of API
objects use
[camel case](https://en.wikipedia.org/wiki/Camel_case).

Don't split the API object name into separate words. For example, use
PodTemplateList, not Pod Template List.

Refer to API objects without saying "object," unless omitting "object"
leads to an awkward sentence construction.

<table caption="Do and Don't - API objects" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>The Pod has two containers.</td><td>The pod has two containers.</td></tr>
<tr><td>The Deployment is responsible for ...</td><td>The Deployment object is responsible for ...</td></tr>
<tr><td>A PodList is a list of Pods.</td><td>A Pod List is a list of pods.</td></tr>
<tr><td>The two ContainerPorts ...</td><td>The two ContainerPort objects ...</td></tr>
<tr><td>The two ContainerStateTerminated objects ...</td><td>The two ContainerStateTerminateds ...</td></tr>
</table>

### Use angle brackets for placeholders

Use angle brackets for placeholders. Tell the reader what a placeholder represents.

1. Display information about a Pod:

        kubectl describe pod <pod-name> -n <namespace>

    If the pod is in the default namespace, you can omit the '-n' parameter.

### Use bold for user interface elements

<table caption="Do and Don't - Bold interface elements" markdown="1">
<tr><th>Do</th><th>Don't</th></tr>
<tr><td markdown="span">Click **Fork**.</td><td>Click "Fork".</td></tr>
<tr><td markdown="span">Select **Other**.</td><td>Select "Other".</td></tr>
</table>

### Use italics to define or introduce new terms

<table caption="Do and Don't - Use italics for new terms" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td markdown="span">A _cluster_ is a set of nodes ...</td><td>A "cluster" is a set of nodes ...</td></tr>
<tr><td markdown="span">These components form the _control plane_.</td><td markdown="span">These components form the **control plane**.</td></tr>
</table>

### Use code style for filenames, directories, paths, object field names and namespaces

<table caption="Do and Don't - Use code style for filenames, directories, paths, object field names and namespaces" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td markdown="span">Open the `envars.yaml` file.</td><td>Open the envars.yaml file.</td></tr>
<tr><td markdown="span">Go to the `/docs/tutorials` directory.</td><td>Go to the /docs/tutorials directory.</td></tr>
<tr><td markdown="span">Open the `/_data/concepts.yaml` file.</td><td markdown="span">Open the /\_data/concepts.yaml file.</td></tr>
</table>

### Use punctuation inside quotes

<table caption="Do and Don't - Use punctuation inside quotes" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>events are recorded with an associated "stage."</td><td>events are recorded with an associated "stage".</td></tr>
<tr><td>The copy is called a "fork."</td><td>The copy is called a "fork".</td></tr>
</table>

Exception: When the quoted word is a user input.

Example:
* My user ID is “IM47g”.
* Did you try the password “mycatisawesome”?

## Inline code formatting


### Use code style for inline code and commands

For inline code in an HTML document, use the `<code>` tag. In a Markdown
document, use the backtick (`` ` ``).

<table caption="Do and Don't - Use code style for inline code and commands" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td markdown="span">The `kubectl run` command creates a Deployment.</td><td>The "kubectl run" command creates a Deployment.</td></tr>
<tr><td markdown="span">For declarative management, use `kubectl apply`.</td><td>For declarative management, use "kubectl apply".</td></tr>
<tr><td markdown="span">Use single backticks to enclose inline code. For example, `var example = true`.</td><td>Use two asterisks (`**`) or an underscore (`_`) to enclose inline code. For example, **var example = true**.</td></tr>
<tr><td>Use triple backticks (\`\`\`) before and after a multi-line block of code for fenced code blocks.</td><td>Use multi-line blocks of code to create diagrams, flowcharts, or other illustrations.</td></tr>
<tr><td>Use meaningful variable names that have a context.</td><td>Use variable names such as 'foo','bar', and 'baz' that are not meaningful and lack context.</td></tr>
<tr><td>Remove trailing spaces in the code.</td><td>Add trailing spaces in the code, where these are important, because a screen reader will read out the spaces as well.</td></tr>
</table>

### Starting a sentence with a component tool or component name

<table caption="Do and Don't - Starting a sentence with a component tool or component name" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td markdown="span">The `kubeadm` tool bootstraps and provisions machines in a cluster.</td><td markdown="span">`kubeadm` tool bootstraps and provisions machines in a cluster.</td></tr>
<tr><td>The kube-scheduler is the default scheduler for Kubernetes.</td><td>kube-scheduler is the default scheduler for Kubernetes.</td></tr>
</table>

### Use normal style for string and integer field values

For field values of type string or integer, use normal style without quotation marks.

<table caption="Do and Don't - Use normal style for string and integer field values" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td markdown="span">Set the value of `imagePullPolicy` to `Always`.</td><td markdown="span">Set the value of `imagePullPolicy` to "Always".</td></tr>
<tr><td markdown="span">Set the value of `image` to `nginx:1.16`.</td><td markdown="span">Set the value of `image` to nginx:1.16.</td></tr>
<tr><td markdown="span">Set the value of the `replicas` field to `2`.</td><td markdown="span">Set the value of the `replicas` field to 2.</td></tr>
</table>

## Code snippet formatting


### Don't include the command prompt

<table caption="Do and Don't - Don't include the command prompt" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>kubectl get pods</td><td>$ kubectl get pods</td></tr>
</table>

### Separate commands from output

Verify that the Pod is running on your chosen node:

```
kubectl get pods --output=wide
```

The output is similar to this:

```
NAME     READY     STATUS    RESTARTS   AGE    IP           NODE
nginx    1/1       Running   0          13s    10.200.0.4   worker0
```

## Velero.io word list


A list of Velero-specific terms and words to be used consistently across the site.

<table caption="Velero.io word list" >
<tr><th>Trem</th><th>Useage</th></tr>
<tr><td>Kubernetes</td><td>Kubernetes should always be capitalized.</td></tr>
<tr><td>Docker</td><td>Docker should always be capitalized.</td></tr>
<tr><td>Velero</td><td>Velero should always be capitalized.</td></tr>
<tr><td>VMware</td><td>VMware should always be correctly capitalized.</td></tr>
<tr><td>On-premises</td><td>On-premises or on-prem rather than on-premise or other variations.</td></tr>
<tr><td>Backup</td><td>Backup rather than back up, back-up or other variations.</td></tr>
<tr><td>Plugin</td><td>Plugin rather than plug-in or other variations.</td></tr>
<tr><td>Allowlist</td><td>Use allowlist instead of whitelist.</td></tr>
<tr><td>Denylist</td><td>Use denylist instead of blacklist.</td></tr>
</table>

## Markdown elements

### Headings
People accessing this documentation may use a screen reader or other assistive technology (AT). [Screen readers](https://en.wikipedia.org/wiki/Screen_reader) are linear output devices, they output items on a page one at a time. If there is a lot of content on a page, you can use headings to give the page an internal structure. A good page structure helps all readers to easily navigate the page or filter topics of interest.

<table caption="Do and Don't - Headings" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>Include a title on each page or blog post.</td><td>Include more than one title headings (#) in a page.</td></tr>
<tr><td>Use ordered headings to provide a meaningful high-level outline of your content.</td><td>Use headings level 4 through 6, unless it is absolutely necessary. If your content is that detailed, it may need to be broken into separate articles.</td></tr>
<tr><td markdown="span">Use sentence case for headings. For example, **Extend kubectl with plugins**</td><td markdown="span">Use title case for headings. For example, **Extend Kubectl With Plugins**</td></tr>
</table>

### Paragraphs

<table caption="Do and Don't - Paragraphs" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td>Try to keep paragraphs under 6 sentences.</td><td>Write long-winded paragraphs.</td></tr>
<tr><td markdown="span">Use three hyphens (`---`) to create a horizontal rule for breaks in paragraph content.</td><td>Use horizontal rules for decoration.</td></tr>
</table>

### Links

<table caption="Do and Don't - Links" >
<tr><th>Do</th><th>Don't</th></tr>
<tr><td markdown="span">Write hyperlinks that give you context for the content they link to. For example: Certain ports are open on your machines. See [check required ports](#check-required-ports) for more details.</td><td markdown="span">Use ambiguous terms such as “click here”. For example: Certain ports are open on your machines. See [here](#check-required-ports) for more details.</td></tr>
<tr><td markdown="span">Write Markdown-style links: `[link text](URL)`. For example: `[community meeting agenda](https://hackmd.io/Jq6F5zqZR7S80CeDWUklkA)` and the output is  [community meeting agenda](https://hackmd.io/Jq6F5zqZR7S80CeDWUklkA).</td><td markdown="span">Write HTML-style links: `<a href="/media/examples/link-element-example.css" target="_blank">Visit our tutorial!</a>`</td></tr>
</table>


### Lists

Group items in a list that are related to each other and need to appear in a specific order or to indicate a correlation between multiple items. When a screen reader comes across a list—whether it is an ordered or unordered list—it will be announced to the user that there is a group of list items. The user can then use the arrow keys to move up and down between the various items in the list.
Website navigation links can also be marked up as list items; after all they are nothing but a group of related links.

 - End each item in a list with a period if one or more items in the list are complete sentences. For the sake of consistency, normally either all items or none should be complete sentences.

  - Ordered lists that are part of an incomplete introductory sentence can be in lowercase and punctuated as if each item was a part of the introductory sentence.

 - Use the number one (`1.`) for ordered lists.

 - Use (`+`), (`*`), or (`-`) for unordered lists - be consistent within the same document.

 - Leave a blank line after each list.

 - Indent nested lists with four spaces (for example, ⋅⋅⋅⋅).

 - List items may consist of multiple paragraphs. Each subsequent paragraph in a list item must be indented by either four spaces or one tab.

### Tables

The semantic purpose of a data table is to present tabular data. Sighted users can quickly scan the table but a screen reader goes through line by line. A table caption is used to create a descriptive title for a data table. Assistive technologies (AT) use the HTML table caption element to identify the table contents to the user within the page structure. For example, `<table caption="Do and Don't - Use present tense">`. To make tables accessible, use HTML formatting to create tables.

If you need to create a table and style table data with markdown, add `markdown="span"` to all `<td>` where the markdown interpreter should be applied.
