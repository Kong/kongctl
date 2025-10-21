---
title: "Example Guides Page Header"
description: "An example of a page header reusable snippet with dynamic data."
---

<!--
Example Usage:

::snippet
---
name: "example-guides-page-header"
data:
  tagline: "Guides" # Optional tagline
  title: "Document APIs"
  description: "Discover best practices, tools, and examples to help developers understand and use your APIs with confidence."
---
::
-->

::page-header
---
title-tag: "h1"
---
#tagline
{{ snippet.tagline }}

#title
{{ snippet.title }}

#description
{{ snippet.description }}
::

