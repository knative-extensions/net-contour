---
title: How to enable structured JSON logging
layout: page
---

This document describes how to configure structured logging for Envoy via Contour.

## How the feature works

Contour allows you to choose from a set of JSON fields that will be expanded into Envoy templates and sent to Envoy.
There is a default set of fields if you enable JSON logging, and you may customize which fields you log.
Custom fields are not currently possible, however, we welcome PRs on the field list.

The canonical location for the current field list is at [JSONFields][1].
The default list of fields is available at [DefaultFields][2]

## Enabling the feature

To enable the feature you have two options:

- just add `--accesslog-format=json` to your Contour startup line
- Add `accesslog-format: json` to your configuration file.

## Customizing logged fields

To customize the logged fields, add a `json-fields` list of strings to your config file.
These strings must be options from the [list of valid fields][1].
If the `json-fields` key is not specified, the [default fields][2] will be configured.
Field names not in that list will be silently dropped. (This is not ideal, watch [#1507][3] for updates.)

The [example config file][4] contains the full list of fields as well.

## Sample configuration file

Here is a sample config:

```yaml
accesslog-format: json
json-fields:
  - "@timestamp"
  - "authority"
  - "bytes_received"
  - "bytes_sent"
  - "downstream_local_address"
  - "downstream_remote_address"
  - "duration"
  - "method"
  - "path"
  - "protocol"
  - "request_id"
  - "requested_server_name"
  - "response_code"
  - "response_flags"
  - "uber_trace_id"
  - "upstream_cluster"
  - "upstream_host"
  - "upstream_local_address"
  - "upstream_service_time"
  - "user_agent"
  - "x_forwarded_for"
```

[1]: https://godoc.org/github.com/projectcontour/contour/internal/envoy#JSONFields
[2]: https://godoc.org/github.com/projectcontour/contour/internal/envoy#DefaultFields
[3]: {{site.github.repository_url}}/issues/1507
[4]: {{site.github.repository_url}}/blob/{{site.github.latest_release.tag_name}}/examples/contour/01-contour-config.yaml
