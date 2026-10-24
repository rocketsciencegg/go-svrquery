# SvrSample package

This server sample package provides sample libraries for providing basic responses to server query requests.
It is not designed to implement the full specs of protocols, however it will provide enough to respond to basic
server information requests.

The sample implementation here will be enough to satisfy the requirements for Multiplay's scaling system to query
the server for health, player counts and other useful information.

The SQP responder accepts wire versions 1 and 2 and echoes the requested version
in its response. It serves server information and metrics when requested. Use
the `sqp` client for version 1 server information, or `sqp-v2` for version 2
server information and metrics.
