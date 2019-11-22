# vnet
vnet is the virtual network layer for Pion. This allows developers to simulate issues that cause issues
with production WebRTC deployments.

See the full documentation for vnet [here](https://github.com/pion/transport/tree/master/vnet#vnet)

## What can vnet do
* Simulate different network topologies. Assert when a STUN/TURN server is actually needed.
* Simulate packet loss, jitter, re-ordering. See how your application performs under adverse conditions.
* Measure the total bandwidth used. Determine the total cost of running your application.
* More! We would love to continue extending this to support everyones needs.

## Instructions
Each directory contains a single `main.go` that aims to demonstrate a single feature of vnet.
They can all be run directly, and require no additional setup.
