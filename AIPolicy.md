# Velero AI Policy

## Summary
Assistive AI tools are permitted, as long as contributions are marked in the commit with Assisted-By or Generated-By messages.

## Accountablity 
* A human must always be in the loop. Contributors need to fully understand and be able to debug any AI-generated code they include. Treat such code as if it came from an untrusted source. 
* Reviewers should apply heightened scrutiny to AI-generated content. Copyright law continues to apply to all preexisting works.
* A human is ultimately responsible for any submitted contribution.

## Policy
### Submissions:
* In your contributions to the Velero project the submission's commit message must contain either
“Assisted-By $AI” or  “Generated-By $AI”. Which label used is the developers preference, both will be treated the same by maintainers. 
* For example: 
  * "Assisted-By: Claude AI"
  * "Generated-By: DeepSeek AI"
  * "Co-Authored-By: AI <ai@example.com>"
### Reviews:
* When reviewing contributions with the “Generated-By” or “Assisted-By labels, verify that the change includes sufficient explanation of the context that the reviewer and future contributors can understand the purpose and origin.
* Apply a higher level of scrutiny to contributions created using AI tools, understanding the limitations of the tools. This does not mean automatically rejecting all contributions that use AI tools, it means giving them the same consideration of technical and legal merits and standards as you would give to any other change.
* Code style changes may be necessary to meet project standards and community guidelines, please work with the contributor as-needed

