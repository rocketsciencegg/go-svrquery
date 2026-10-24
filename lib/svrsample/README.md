# SvrSample package

This server sample package provides sample libraries for providing basic responses to server query requests.
It is not designed to implement the full specs of protocols, however it will provide enough to respond to basic
server information requests.

The sample implementation here will be enough to satisfy the requirements for Multiplay's scaling system to query
the server for health, player counts and other useful information.

`GetResponder` supports `sqp`, `tf2` (legacy version 1), `tf2e` (version 3),
`tf2e-v7`, and `tf2e-v8`. TF2E requests must be unkeyed; the sample does not
implement authentication or encryption, nor the version 9 or 10 schemas.
The query client supports the TF2E variants, but not legacy `tf2`.
